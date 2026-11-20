# Dashboard Geographic Distribution

The overview page displays statistical metrics directly below a single-line title and map legend. Data automatically refreshes every 60 seconds.

The top row of the global dashboard displays metrics for the last 24 hours: total requests, unique IPs, 4xx requests, and 5xx requests. Total requests include the average QPS. The 4xx and 5xx metrics display their respective percentages of total requests. Unique IPs are deduplicated across all nodes. All metrics derive from persisted access logs; when there are no requests, error percentages display as 0%.

The global dashboard displays a dot-matrix 3D globe by default. Drag the globe or use arrow keys when focused to view different regions. Click a node marker to navigate to node details; hover over a node to display request counts, error counts, and health status. Node marker colors correspond to the dashboard legend for normal, loaded, and degraded states.

Use the **3D / 2D** toggle in the top-right corner to switch between the 3D globe and a 2D flat projection. Both views color countries according to inbound request volumes over the last 24 hours, with deeper colors representing higher request counts. Sources with geographic coordinates but no polygon boundary data display as independent heat markers. The flat map supports dragging and zooming; hover over regions or heat markers to inspect request numbers. The 3D view rotates by default toward the country with the highest resolvable request volume; manual rotation angles persist across automatic 60-second refreshes.

The panel to the right of the map displays the top seven source countries by request volume over the last 24 hours, with progress bars scaled against the top source country. An empty state displays when no source data is available.

Both 3D and 2D views share the same container height to prevent layout shifts during mode switches. Map colors adapt to the active interface theme and light/dark modes. On narrow viewports, the country ranking moves below the map container.

Geographic boundaries use embedded dataset files. Nodes without configured coordinates fall back to country centroids or preset coordinates, and hover tooltips indicate fallback coordinates.
