use super::{Level, Levels, RuleMatch, CATEGORIES};
use anyhow::{ensure, Context, Result};
use include_dir::{include_dir, Dir};
use std::{collections::HashMap, fs};
use zentinel_modsec::{
    parser::{Action, Directive, MetadataAction, OperatorName, Parser, RuleIdSelector, SecRule},
    CompiledRuleset,
};

static CRS_RULES: Dir<'_> = include_dir!("$CARGO_MANIFEST_DIR/vendor/owasp-crs/rules");

const SETUP: &str = r#"
SecRuleEngine DetectionOnly
SecRequestBodyAccess On
SecAction "phase:1,pass,nolog,setvar:tx.crs_setup_version=4300,setvar:tx.detection_paranoia_level=4"
"#;

const SUPPLEMENTAL: &str = r#"
SecRule REQUEST_BODY "@rx <!entity\s+[^>]*\b(?:system|public)\s+" "id:11071,phase:2,pass,nolog,t:lowercase,msg:'XML external entity declaration'"
SecRule REQUEST_BODY "@rx <!doctype\s+[^>]*\b(?:system|public)\s+|<!entity\s" "id:11072,phase:2,pass,nolog,t:lowercase,msg:'XML DTD or entity declaration'"
SecRule ARGS|ARGS_NAMES|REQUEST_COOKIES|REQUEST_URI|REQUEST_BODY|REQUEST_HEADERS:Referer "@rx <%\s*(?:@\s*page\b|response\s*\.|server\s*\.)" "id:11131,phase:2,pass,nolog,t:urlDecodeUni,t:htmlEntityDecode,t:lowercase,msg:'ASP server-side code block'"
SecRule ARGS|ARGS_NAMES|REQUEST_COOKIES|REQUEST_URI|REQUEST_BODY|REQUEST_HEADERS:Referer "@rx <%[=!]|\bserver\.createobject\s*\(" "id:11132,phase:2,pass,nolog,t:urlDecodeUni,t:htmlEntityDecode,t:lowercase,msg:'ASP expression or object creation'"
"#;

pub(super) struct Rules {
    directives: Vec<Directive>,
    _data_dir: tempfile::TempDir,
}

impl Rules {
    pub(super) fn load() -> Result<Self> {
        let data_dir = tempfile::tempdir().context("create OWASP CRS data directory")?;
        let mut configs = Vec::new();
        for file in CRS_RULES.files() {
            match file
                .path()
                .extension()
                .and_then(|extension| extension.to_str())
            {
                Some("data") => fs::write(data_dir.path().join(file.path()), file.contents())
                    .with_context(|| format!("extract CRS data {}", file.path().display()))?,
                Some("conf") => configs.push(file),
                _ => {}
            }
        }
        configs.sort_unstable_by_key(|file| file.path());
        let mut parser = Parser::new();
        parser.parse(SETUP)?;
        for file in configs {
            let path = data_dir.path().join(file.path());
            let source = file.contents_utf8().context("CRS file must be UTF-8")?;
            parser.parse_with_location(source, Some(&path))?;
        }
        parser.parse(SUPPLEMENTAL)?;
        let directives = parser.into_directives();
        for directive in &directives {
            if let Directive::SecRule(rule) = directive {
                if matches!(
                    rule.operator.name,
                    OperatorName::PmFromFile | OperatorName::IpMatchFromFile
                ) {
                    let path = std::path::Path::new(&rule.operator.argument);
                    ensure!(
                        path.starts_with(data_dir.path())
                            && path.is_file()
                            && !rule.operator.argument.contains("%{"),
                        "invalid embedded CRS data reference at {}",
                        rule.location
                    );
                }
            }
        }
        Ok(Self {
            directives,
            _data_dir: data_dir,
        })
    }

    pub(super) fn compile(
        &self,
        levels: Levels,
    ) -> Result<(CompiledRuleset, HashMap<String, RuleMatch>)> {
        let mut directives = self.directives.clone();
        let mut removals = Vec::new();
        let mut matches = HashMap::new();
        for directive in &directives {
            let Directive::SecRule(rule) = directive else {
                continue;
            };
            let Some(id) = rule.id() else { continue };
            let paranoia = paranoia_level(rule)?;
            let Some(category) = category(rule, id) else {
                if paranoia.is_some() {
                    removals.push(RuleIdSelector::Single(id));
                }
                continue;
            };
            let paranoia = match id {
                11071 | 11131 => 1,
                11072 | 11132 => 2,
                _ => paranoia.with_context(|| format!("CRS rule {id} has no paranoia level"))?,
            };
            let level = match levels[CATEGORIES
                .iter()
                .position(|name| *name == category)
                .unwrap()]
            {
                Level::Off => None,
                Level::Balanced if paranoia > 1 => None,
                Level::Balanced => Some("balanced"),
                Level::Strict => Some("strict"),
                Level::Observe => Some("observe"),
            };
            let Some(level) = level else {
                removals.push(RuleIdSelector::Single(id));
                continue;
            };
            let message = rule
                .actions
                .iter()
                .find_map(|action| match action {
                    Action::Metadata(MetadataAction::Msg(message)) => Some(message.clone()),
                    _ => None,
                })
                .with_context(|| format!("attack rule {id} has no message"))?;
            let previous = matches.insert(
                id.to_string(),
                RuleMatch {
                    id: u32::try_from(id)?,
                    category: category.into(),
                    message,
                    level: level.into(),
                },
            );
            ensure!(previous.is_none(), "duplicate attack rule ID {id}");
        }
        directives.push(Directive::SecRuleRemoveById(removals));
        let ruleset = CompiledRuleset::compile(directives).context("compile embedded OWASP CRS")?;
        Ok((ruleset, matches))
    }
}

fn tags(rule: &SecRule) -> impl Iterator<Item = &str> {
    rule.actions.iter().filter_map(|action| match action {
        Action::Metadata(MetadataAction::Tag(tag)) => Some(tag.as_str()),
        _ => None,
    })
}

fn category(rule: &SecRule, id: u64) -> Option<&'static str> {
    Some(match id {
        11071 | 11072 => "xxe",
        11131 | 11132 => "asp_injection",
        921120 | 921130 | 921140 | 921150 | 921151 | 921160 | 921190 => "crlf_injection",
        922100 | 922110 | 922120 | 922130 | 932180 | 933110 | 933111 | 933220 | 944140 => {
            "malicious_upload"
        }
        930100 | 930110 => "path_traversal",
        930120..=930999 | 931100..=931999 => "file_inclusion",
        932100..=932999 => "command_injection",
        933170 => "php_deserialization",
        933100..=933999 => "php_injection",
        934100..=934999 if tags(rule).any(|tag| tag == "attack-ssrf") => "ssrf",
        934100..=934999 if tags(rule).any(|tag| tag == "attack-ssti") => "template_injection",
        941100..=941999 => "xss",
        942100..=942999 => "sql_injection",
        944120 | 944200 | 944210 | 944240 => "java_deserialization",
        944100..=944999 => "java_injection",
        _ => return None,
    })
}

fn paranoia_level(rule: &SecRule) -> Result<Option<u8>> {
    let Some(tag) = tags(rule).find_map(|tag| tag.strip_prefix("paranoia-level/")) else {
        return Ok(None);
    };
    let paranoia = tag.parse::<u8>()?;
    ensure!(
        (1..=4).contains(&paranoia),
        "invalid CRS paranoia level at {}",
        rule.location
    );
    Ok(Some(paranoia))
}
