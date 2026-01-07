# Automating SQL Server Diagnostic Collection with Go and Excel Output

A command line tool for collecting SQL Server diagnostic information and generating Excel reports for offline analysis.

Code: [Get SQL Server Diagnostics](https://github.com/MalcolmPereira/getSQLServerDiagnostics)

---

## Diagnosing SQL Server Issues Remotely

In an ideal world, one would have direct access to the server, SQL Server Management Studio open, and hours to methodically investigate. But reality is often different. Maybe you are supporting a client installation remotely. Perhaps the database is behind strict firewall rules, and you only have limited connection windows, Or you're part of a consulting team that needs to gather diagnostic data from dozens of SQL Server instances across different environments. The application is not behaving well and issues seem to stem to the database. We need a way to gather and database diagnostic data for analysis.

Traditional troubleshooting approaches have limitations in these scenarios:

**Real Time monitoring tools** like SQL Server Profiler or Extended Events are excellent for live analysis, but they require persistent connections and hands on interaction. When you are working across time zones or dealing with intermittent issues, being always on is not practical.

**Reports and Dashboards** provide useful summaries, but often lack the depth needed for serious troubleshooting. You end up running the same diagnostic queries manually, copying results to spreadsheets, and repeating this process for each server.

**Remote Desktop sessions** work but are bandwidth heavy, require constant connectivity, and do not scale when you need to collect data from multiple servers.

What if you could capture a comprehensive snapshot of your SQL Server's health configuration settings, performance counters, wait statistics, index usage, query plans, and more all in a single Excel file that you can analyze offline, share with colleagues, or archive for trend analysis ?

This SQL Server Diagnostic Collection tool can help, it comes will some well proven samples and you can plug in your own as needed.

## Acknowledgments

This tool stands on the shoulders of giants in the SQL Server community:

[Glenn Berry](https://glennsqlperformance.com/)

- For his meticulously maintained SQL Server Diagnostic Information Queries, updated for each SQL Server version and widely used by DBAs worldwide.

[Adam Machanic](http://whoisactive.com/)

- For sp_whoisactive, the essential stored procedure for understanding what is currently running on your SQL Server.

[Brent Ozar](https://www.brentozar.com/)

- For his extensive library of troubleshooting scripts and educational resources that help DBAs solve real world performance problems.

Their freely shared expertise makes tools like this possible.

## How This Tool Helps

**getSQLServerDiagnostics** is a lightweight Go application designed to solve exactly this problem. It connects to a SQL Server instance, executes a curated set of diagnostic queries, and outputs everything to a timestamped Excel workbook.

The approach is simple but powerful:

1. **Define your queries once** in a JSON configuration file
2. **Run the tool** against any SQL Server instance
3. **Receive a comprehensive Excel report** with each query's results on a separate worksheet
4. **Analyze offline** at your convenience

This tool leverages the excellent diagnostic queries developed by SQL Server experts like [Glenn Berry](https://glennsqlperformance.com/), whose SQL Server Diagnostic Information Queries have become an industry standard for performance troubleshooting. It also supports [Adam Machanic's sp_whoisactive](http://whoisactive.com/) and queries inspired by [Brent Ozar's troubleshooting scripts](https://www.brentozar.com/).

By packaging these expert level queries into an automated collection process, you get consistent, repeatable diagnostic snapshots without manually copying and pasting results.

### Key Benefits

- **Offline analysis**: Collect data once, analyze anywhere without maintaining a live connection
- **Consistency**: Same queries run the same way every time, making it easy to compare snapshots over time
- **Portability**: A single Excel file is easy to share, email, or attach to support tickets
- **Flexibility**: Bring your own queries or use the pre built query sets for different SQL Server versions
- **Scheduling**: Run continuously at intervals for monitoring (e.g., every 5 minutes for 24 hours)
- **No agent required**: Just needs a database connection no software installation on the server

---

## Understanding the Tool's Architecture

The tool is built around three main components that work together:

### 1. Configuration File (`config.properties`)

This is where you define how to connect to your SQL Server instance. The tool supports multiple authentication methods:

```properties
# Basic connection details
DB_HOST=your-server.database.windows.net
DB_PORT=1433
DB_NAME=master

# SQL Server authentication
USER=diagnostic_user
PASSWORD=your_password
TRUSTED=false

# Or use Windows integrated security
TRUSTED=true

# Or provide a complete custom connection string
USER_DEFINED=sqlserver://user:pass@host:1433?database=master&encrypt=true
```

The `USER_DEFINED` option gives you full control over the connection string, which is particularly useful for Azure SQL Database or instances with specific encryption requirements.

### 2. Query Definition Files (JSON)

Queries are defined in JSON files, making them easy to version control, share, and customize. The tool ships with several pre built query sets:

| File | Purpose |
|------|---------|
| `sql_queries.json` | Default queries for SQL Server 2022 |
| `sql_queries_2022.json` | SQL Server 2022 specific diagnostics |
| `sql_queries_2025.json` | SQL Server 2025 specific diagnostics |
| `sql_queries_azure.json` | Adapted queries for Azure SQL Database |
| `sql_queries_custom.json` | Template for your own custom queries |
| `sql_queries_spwhoisactive.json` | Queries using sp_whoisactive |

Each query in the JSON file includes metadata that helps organize the output:

```json
{
  "queries": [
    {
      "name": "WaitStats",
      "description": "Top waits by percentage - identifies bottlenecks",
      "query": "SELECT wait_type, wait_time_ms, ... FROM sys.dm_os_wait_stats ...",
      "notes": "Focus on top 5 wait types for initial investigation"
    }
  ]
}
```

The `name` field becomes the Excel worksheet name (limited to 31 characters per Excel's requirements), while `description` and `notes` are captured in a summary sheet for reference.

### 3. The Go Application (`app.go`)

The application itself is a single Go file that orchestrates the entire process:

1. Parses command-line arguments for configuration and query file paths
2. Displays a safety prompt (you must type "yes" to proceed—this ensures you've reviewed the queries)
3. Reads the database configuration and establishes a connection
4. Loads and executes each query sequentially
5. Writes results directly to Excel worksheets (no intermediate CSV files)
6. Saves the output as `sql_diagnostics_DDMMYYYY_HHMMSS.xlsx`

The direct-to-Excel approach means faster execution and less disk I/O compared to tools that create temporary files.

---

## Running the Tool

### Prerequisites

- Go 1.24 or later installed
- Network access to your SQL Server instance
- A SQL Server login with appropriate permissions to run diagnostic queries.

### Quick Start

1. **Clone the repository** and navigate to the project directory

2. **Install dependencies**:

   ```bash
   go mod tidy
   ```

3. **Configure your connection** by copying the template:

   ```bash
   cp config.properties_template config.properties
   ```

   Edit `config.properties` with your server details.

4. **Review the query file** you plan to use (e.g., `sql_queries.json`) to understand what will be executed.

5. **Run the tool**:

   ```bash
   go run app.go
   ```

   Type `yes` when prompted to confirm you've reviewed the queries.

6. **Find your results** in the generated Excel file (e.g., `sql_diagnostics_07012026_143022.xlsx`)

### Command-Line Options

```bash
# Use a specific configuration file
go run app.go -config="production_config.properties"

# Use a specific query set
go run app.go -queries="sql_queries_azure.json"

# Combine both
go run app.go -config="azure_config.properties" -queries="sql_queries_azure.json"

# Run repeatedly for monitoring (every 5 minutes for 24 hours)
go run app.go -interval=5 -duration=24
```

The interval/duration option is particularly useful for capturing data during a known problem window or for gathering baseline metrics over time.

### Building a Standalone Binary

For deployment to systems without Go installed:

```bash
# Windows
go build -o getSQLServerDiagnostics.exe

# Linux/macOS
go build -o getSQLServerDiagnostics
```

---

## Creating Custom Query Sets

While the included query sets cover most common diagnostic scenarios, you can create your own. Start by copying `sql_queries_custom.json` and modifying it:

```json
{
  "querysource": {
    "sqlserverversion": "2022",
    "name": "My Custom Diagnostics",
    "author": "Your Name",
    "lastmodified": "2026-01-07"
  },
  "queries": [
    {
      "name": "ActiveConnections",
      "description": "Current active connections by database",
      "query": "SELECT DB_NAME(dbid) as DatabaseName, COUNT(*) as Connections FROM sys.sysprocesses WHERE dbid > 0 GROUP BY dbid ORDER BY COUNT(*) DESC",
      "notes": "Useful for identifying connection leaks"
    }
  ]
}
```

Tips for custom queries:

- Keep query names under 31 characters (Excel worksheet limit)
- Avoid special characters in names: `\ / ? * [ ] :`
- Format queries as single lines in the JSON (escape special characters as needed)
- Test queries manually before adding them to ensure they work on your SQL Server version
