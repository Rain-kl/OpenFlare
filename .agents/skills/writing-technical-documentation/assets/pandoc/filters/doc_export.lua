-- Pandoc Lua filter for technical documents. Written for pandoc 3.1.10 or later; tested on 3.12.1.
--
--   GitHub alerts      > [!NOTE] blocks become styled callouts in Word and PDF.
--   Page breaks        <!-- pagebreak --> starts a new page in every format.
--   Repo-relative links   Links such as ../src/app.py are dead outside the repo, so they become
--                      absolute when metadata doc-export-link-base is set, and plain text if not.
--   Mermaid            ```mermaid blocks render to images through mmdc, with alt text taken from
--                      accTitle and accDescr. Without mmdc the source stays as a code block.
--
-- Set metadata doc-export-mermaid to "off" to leave Mermaid source untouched. Set MERMAID_BIN to
-- use a specific mmdc, and MERMAID_PUPPETEER_CONFIG for a Puppeteer config file, which CI usually
-- needs. Warnings go to stderr with the prefix [doc_export].

local ALERT_KINDS = { note = true, tip = true, important = true, warning = true, caution = true }
local ALERT_FILL = { note = "#eef3f8", tip = "#eef6ee", important = "#f3eef8", warning = "#fbf3e6", caution = "#fbeaea" }

local link_base = nil
local mermaid_enabled = true
local warned = {}

local function warn_once(key, message)
  if not warned[key] then
    warned[key] = true
    io.stderr:write("[doc_export] " .. message .. "\n")
  end
end

local function is_format(pattern)
  return FORMAT:match(pattern) ~= nil
end

local function read_meta(meta)
  if meta["doc-export-link-base"] then
    link_base = pandoc.utils.stringify(meta["doc-export-link-base"]):gsub("/+$", "")
  end
  if meta["doc-export-mermaid"] and pandoc.utils.stringify(meta["doc-export-mermaid"]) == "off" then
    mermaid_enabled = false
  end
end

-- Alerts ------------------------------------------------------------------------------------------

local function alert_kind(div)
  for _, class in ipairs(div.classes) do
    if ALERT_KINDS[class] then
      return class
    end
  end
  return nil
end

local function split_alert(div)
  local title, body = nil, pandoc.List()
  for _, block in ipairs(div.content) do
    if block.t == "Div" and block.classes:includes("title") and not title then
      title = block.content
    else
      body:insert(block)
    end
  end
  return title, body
end

local function style_alert(div)
  local kind = alert_kind(div)
  if not kind then
    return nil
  end
  local title, body = split_alert(div)
  if not title then
    title = { pandoc.Para({ pandoc.Str(kind:sub(1, 1):upper() .. kind:sub(2)) }) }
  end
  if is_format("docx") then
    local out = pandoc.List({ pandoc.Div(title, { ["custom-style"] = "Callout Title" }) })
    out:insert(pandoc.Div(body, { ["custom-style"] = "Callout" }))
    return out
  end
  if is_format("typst") then
    local open = string.format('#block(fill: rgb("%s"), inset: 8pt, radius: 2pt, width: 100%%)[', ALERT_FILL[kind])
    local out = pandoc.List({ pandoc.RawBlock("typst", open) })
    out:insert(pandoc.Para({ pandoc.Strong(pandoc.utils.blocks_to_inlines(title)) }))
    out:extend(body)
    out:insert(pandoc.RawBlock("typst", "]"))
    return out
  end
  if is_format("latex") then
    local out = pandoc.List({ pandoc.Para({ pandoc.Strong(pandoc.utils.blocks_to_inlines(title)) }) })
    out:extend(body)
    return pandoc.BlockQuote(out)
  end
  return nil
end

-- Page breaks -------------------------------------------------------------------------------------

local function page_break()
  if is_format("docx") then
    return pandoc.RawBlock("openxml", '<w:p><w:r><w:br w:type="page"/></w:r></w:p>')
  elseif is_format("typst") then
    return pandoc.RawBlock("typst", "#pagebreak()")
  elseif is_format("latex") then
    return pandoc.RawBlock("latex", "\\newpage{}")
  elseif is_format("html") then
    return pandoc.RawBlock("html", '<div class="page-break"></div>')
  end
  return pandoc.HorizontalRule()
end

local function raw_block(el)
  if el.format:match("html") and el.text:match("^%s*<!%-%-%s*pagebreak%s*%-%->%s*$") then
    return page_break()
  end
  return nil
end

-- Links -------------------------------------------------------------------------------------------

local function is_external(target)
  return target:match("^%a[%w+.-]*:") or target:match("^#") or target == ""
end

local function fix_link(el)
  if is_external(el.target) then
    return nil
  end
  if link_base then
    el.target = link_base .. "/" .. el.target:gsub("^%./", "")
    return el
  end
  warn_once("link:" .. el.target, "link to " .. el.target .. " is dead outside the repo; kept the text only. Pass --link-base to keep it.")
  return el.content
end

-- Mermaid -----------------------------------------------------------------------------------------

local function mermaid_alt(source)
  local title = source:match("accTitle:%s*([^\n]+)")
  local descr = source:match("accDescr:%s*([^\n{][^\n]*)") or source:match("accDescr%s*{(.-)}")
  local parts = {}
  if title then
    table.insert(parts, title)
  end
  if descr then
    table.insert(parts, (descr:gsub("%s+", " ")))
  end
  return table.concat(parts, ". ")
end

local function render_mermaid(source)
  local bin = os.getenv("MERMAID_BIN") or "mmdc"
  local name = pandoc.utils.sha1(source) .. ".png"
  local image = nil
  local ok, err = pcall(function()
    pandoc.system.with_temporary_directory("mermaid", function(dir)
      local input = pandoc.path.join({ dir, "in.mmd" })
      local output = pandoc.path.join({ dir, "out.png" })
      local handle = assert(io.open(input, "w"))
      handle:write(source)
      handle:close()
      -- Scale 3 keeps labels sharp at Word's and Typst's page width. White avoids transparent
      -- backgrounds, which render black in some viewers.
      local args = { "--input", input, "--output", output, "--backgroundColor", "white", "--scale", "3", "--quiet" }
      local config = os.getenv("MERMAID_PUPPETEER_CONFIG")
      if config then
        table.insert(args, "--puppeteerConfigFile")
        table.insert(args, config)
      end
      pandoc.pipe(bin, args, "")
      local png = assert(io.open(output, "rb"))
      image = png:read("a")
      png:close()
    end)
  end)
  if not ok or not image then
    warn_once("mermaid", "could not render Mermaid with " .. bin .. "; kept the source as a code block. Install it with: npm install -g @mermaid-js/mermaid-cli (" .. tostring(err) .. ")")
    return nil
  end
  pandoc.mediabag.insert(name, "image/png", image)
  return name
end

local function code_block(el)
  if not el.classes:includes("mermaid") or not mermaid_enabled then
    return nil
  end
  local name = render_mermaid(el.text)
  if not name then
    return nil
  end
  local alt = mermaid_alt(el.text)
  if alt == "" then
    warn_once("mermaid-alt", "a Mermaid diagram has no accTitle or accDescr, so its image has no alt text")
  end
  return pandoc.Para({ pandoc.Image(alt, name) })
end

return {
  { Meta = read_meta },
  { Div = style_alert, RawBlock = raw_block, Link = fix_link, CodeBlock = code_block },
}
