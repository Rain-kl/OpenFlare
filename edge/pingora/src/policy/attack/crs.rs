use super::{Level, Levels, RuleMatch, RuleOverride, RuleOverrides, CATEGORIES};
use anyhow::{ensure, Context, Result};
use include_dir::{include_dir, Dir};
use serde::Serialize;
use std::{collections::HashMap, fs};
use zentinel_modsec::{
    parser::{Action, Directive, MetadataAction, OperatorName, Parser, RuleIdSelector, SecRule},
    CompiledRuleset,
};

static CRS_RULES: Dir<'_> = include_dir!("$CARGO_MANIFEST_DIR/vendor/owasp-crs/rules");

// Explicit presets keep new CRS signatures from starting to block after a catalog update.
// Essential uses specific protocol violations and attack signatures. Balanced adds
// broader signatures while leaving generic keyword and libinjection heuristics to Strict.
const ESSENTIAL_RULES: &[u64] = &[
    11071, 11131, 920160, 920181, 920270, 921110, 921120, 921130, 930100, 930110, 932170, 932171,
    932180, 933110, 933170, 934110, 941110, 941140, 942270, 944110, 944120, 944150,
];

const ADDITIONAL_BALANCED_RULES: &[u64] = &[
    11151, 920100, 920190, 920210, 920470, 920530, 920620, 921140, 921160, 921190, 921421, 922130,
    930120, 930130, 930140, 931110, 932120, 932125, 932140, 932160, 933100, 933140, 933160, 933180,
    933200, 933210, 933220, 934170, 934200, 941120, 941130, 941350, 941360, 942160, 942170, 942190,
    942280, 942320, 942350, 942360, 942540, 944140,
];

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
SecRule TX:openflare_csrf_context "@streq cross_origin" "id:11151,phase:1,pass,nolog,msg:'Cross-origin request with cookies and an unsafe method'"
SecRule TX:openflare_csrf_context "@streq missing_origin" "id:11152,phase:1,pass,nolog,msg:'Request with cookies and an unsafe method has no verifiable origin'"
"#;

#[derive(Serialize)]
pub(super) struct Catalog {
    crs_version: &'static str,
    rules: Vec<RuleInfo>,
}

#[derive(Serialize)]
struct RuleInfo {
    id: u32,
    category: &'static str,
    message: String,
    paranoia: Option<u8>,
    essential: bool,
    balanced: bool,
}

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

    pub(super) fn catalog(&self) -> Result<Catalog> {
        let mut rules = Vec::new();
        for directive in &self.directives {
            if let Directive::SecRule(rule) = directive {
                rules.extend(rule_info(rule)?);
            }
        }
        rules.sort_unstable_by_key(|rule| rule.id);
        Ok(Catalog {
            crs_version: "4.30.0",
            rules,
        })
    }

    pub(super) fn compile(
        &self,
        levels: Levels,
        rule_overrides: &RuleOverrides,
    ) -> Result<(CompiledRuleset, HashMap<String, RuleMatch>)> {
        let mut directives = self.directives.clone();
        let mut removals = Vec::new();
        let mut matches = HashMap::new();
        let mut unmatched_overrides = rule_overrides.clone();
        for directive in &directives {
            let Directive::SecRule(rule) = directive else {
                continue;
            };
            let Some(id) = rule.id() else { continue };
            let Some(info) = rule_info(rule)? else {
                if paranoia_level(rule)?.is_some() {
                    removals.push(RuleIdSelector::Single(id));
                }
                continue;
            };
            let level = match unmatched_overrides.remove(&info.id) {
                Some(RuleOverride::Off) => None,
                Some(RuleOverride::Enabled) => Some("enabled"),
                Some(RuleOverride::Observe) => Some("observe"),
                None => match levels[CATEGORIES
                    .iter()
                    .position(|name| *name == info.category)
                    .unwrap()]
                {
                    Level::Off => None,
                    Level::Essential if !info.essential => None,
                    Level::Essential => Some("essential"),
                    Level::Balanced if !info.balanced => None,
                    Level::Balanced => Some("balanced"),
                    Level::Strict => Some("strict"),
                    Level::Observe => Some("observe"),
                },
            };
            let Some(level) = level else {
                removals.push(RuleIdSelector::Single(id));
                continue;
            };
            let previous = matches.insert(
                id.to_string(),
                RuleMatch {
                    id: info.id,
                    category: info.category.into(),
                    message: info.message,
                    level: level.into(),
                },
            );
            ensure!(previous.is_none(), "duplicate attack rule ID {id}");
        }
        ensure!(
            unmatched_overrides.is_empty(),
            "unknown attack protection rule IDs: {:?}",
            unmatched_overrides.keys().collect::<Vec<_>>()
        );
        directives.push(Directive::SecRuleRemoveById(removals));
        let ruleset = CompiledRuleset::compile(directives).context("compile embedded OWASP CRS")?;
        Ok((ruleset, matches))
    }
}

fn rule_info(rule: &SecRule) -> Result<Option<RuleInfo>> {
    let Some(id) = rule.id() else { return Ok(None) };
    let Some(category) = category(rule, id) else {
        return Ok(None);
    };
    let paranoia = match id {
        11071 | 11072 | 11131 | 11132 | 11151 | 11152 => None,
        _ => Some(
            paranoia_level(rule)?
                .with_context(|| format!("CRS rule {id} has no paranoia level"))?,
        ),
    };
    let essential = ESSENTIAL_RULES.contains(&id);
    let message = rule
        .actions
        .iter()
        .find_map(|action| match action {
            Action::Metadata(MetadataAction::Msg(message)) => Some(message.clone()),
            _ => None,
        })
        .with_context(|| format!("attack rule {id} has no message"))?;
    Ok(Some(RuleInfo {
        id: u32::try_from(id)?,
        category,
        message,
        paranoia,
        essential,
        balanced: essential || ADDITIONAL_BALANCED_RULES.contains(&id),
    }))
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
        11151 | 11152 => "csrf",
        920100
        | 920160..=920190
        | 920200..=920202
        | 920210
        | 920230
        | 920240
        | 920260
        | 920270
        | 920271
        | 920280
        | 920290
        | 920460
        | 920470
        | 920510
        | 920521
        | 920530
        | 920610
        | 920620
        | 920660
        | 920275
        | 921110
        | 921421
        | 921422 => "malformed_http",
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
