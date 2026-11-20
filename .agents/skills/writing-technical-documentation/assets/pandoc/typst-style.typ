// Layout for technical documents exported through pandoc's Typst writer.
// Pandoc includes this at the start of the body, so these rules override its template.
// Left-aligned text reads better than justified, and tables need lines and left-aligned cells.
#set par(justify: false)
#set table(stroke: 0.5pt + luma(190), inset: 6pt, fill: (_, y) => if y == 0 { luma(242) })
#show table: set align(left)
#show table.cell.where(y: 0): set text(weight: "bold")
#show raw.where(block: true): block.with(fill: luma(245), inset: 8pt, radius: 2pt, width: 100%)
#show link: set text(fill: rgb("#1F4E79"))
#show heading: set text(fill: rgb("#1F4E79"))
