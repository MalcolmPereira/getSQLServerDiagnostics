/*
Package main

This program connects to a SQL Server database, executes a series of SQL queries defined in a JSON file,
and generates diagnostic reports in Excel format. It uses a configuration file to define database
connection details and dynamically processes queries to produce results directly in Excel worksheets.

The source of the SQL Queries come from https://glennsqlperformance.com/ and we acknowledge this great resource for troubleshooting SQL Server Performance.


Author: Malcolm Pereira
Date: November 27, 2025
Last Modified: November 27, 2025
Revision: 2.0.0

Usage:

- Ensure there exists a `config.properties` file contains the correct database connection details.
  Example:
			DB_HOST=<host name>
			DB_PORT=<port>
			DB_NAME=<database name>
			USER=<user>
			PASSWORD=<password
			TRUSTED=<use integrated security true or false in which case USER and PASSWORD is not needed>

- Define SQL queries in the json file with the following structure.
  Example:
		{
            ...
			...
			...
			"queries": [
					{
						"name": "CheckVersion",
						"description": "Confirm if the SQL Queries will work for the version of SQL Server",
						"query": "IF NOT EXISTS (SELECT * WHERE CONVERT(varchar(128), SERVERPROPERTY('ProductMajorVersion')) = '16') BEGIN DECLARE @ProductVersion varchar(128) = CONVERT(varchar(128), SERVERPROPERTY('ProductVersion')); SELECT SERVERPROPERTY('ProductMajorVersion') AS SERVER_VERSION, 'Script does not match the ProductVersion [%s] of this instance. Many of these queries may not work on this version.' AS MESSAGE END SELECT SERVERPROPERTY('ProductMajorVersion') AS SERVER_VERSION, 'Valid Server Version for the script.' AS MESSAGE",
						"notes":"Confirm if the SQL Queries will work for the version of SQL Server"
					}
					...
					...
					...
			]
		}

- Run the program to generate diagnostic report, that is saved to Excel. The program will directly write query results to Excel worksheets without creating intermediate CSV files, resulting in faster processing and reduced disk I/O.

Dependencies:
	- github.com/microsoft/go-mssqldb for SQL Server connectivity.
	- github.com/xuri/excelize/v2 for Excel file generation.
	- github.com/magiconair/properties for reading configuration files.

Building:
	//Manage Dependencies
	- go mod tidy

	//Build
	- go build -o getSQLServerDiagnostics.exe
	- go build

*/

package main

import (
	// Standard library packages
	"encoding/json" // For parsing and encoding JSON data
	"flag"          // For command line arguments
	"fmt"           // For formatted I/O operations
	"log"           // For logging messages
	"os"            // For interacting with the operating system (e.g., file operations)
	"regexp"        // For working with regular expressions
	"strconv"       // For converting strings to numbers and vice versa
	"strings"       // For string manipulation
	"time"          // For working with date and time

	"database/sql" // Database/sql package for database operations

	// SQL Server driver
	_ "github.com/microsoft/go-mssqldb" // Microsoft SQL Server driver for Go From Microsoft

	// Third-party packages
	"github.com/magiconair/properties" // For reading and handling properties files
	"github.com/xuri/excelize/v2"      // For creating and manipulating Excel files
)

// Default files for config and sql queries
const sql_config = "config.properties" // SQL Server Configuration File
const sql_queries = "sql_queries.json" // SQL Queries File

/*
 * main is the entry point of the application. It initializes the program, parses command-line arguments,
 * and orchestrates the execution of SQL queries and the generation of diagnostic reports.
 *
 * Functionality:
 * 1. Defines command-line flags for specifying the paths to the SQL Server configuration file and the SQL queries JSON file.
 *    - `-config`: Path to the SQL Server configuration file (defaults to `config.properties`).
 *    - `-queries`: Path to the SQL queries JSON file (defaults to `sql_queries.json`).
 * 2. Parses the command-line flags to retrieve the user-specified or default file paths.
 * 3. Logs the start of the application.
 * 4. Calls the `executeSQLQueries` function to:
 *    - Read the SQL Server configuration and queries.
 *    - Execute the queries on the database.
 *    - Save the results to individual CSV files.
 * 5. Calls the `createExcelFromCSV` function to:
 *    - Combine the generated CSV files into a single Excel file.
 *    - Delete the original CSV files after the Excel file is created.
 * 6. Logs the completion of the application.
 *
 * Notes:
 * - The function assumes that the configuration and query files are well-formed and accessible.
 * - The application logs errors and exits gracefully if any critical issues are encountered.
 *
 * Example Usage:
 * Run the program with default file paths:
 *   go run app.go
 *
 * Specify custom file paths:
 *   go run app.go -config="custom_config.properties" -queries="custom_queries.json"
 */
func main() {

	// Define command-line flags
	sqlConfigProp := flag.String("config", sql_config, "Optional: Path to the SQL Server configuration file, defaulting to config.properties if not set.")
	sqlQueries := flag.String("queries", sql_queries, "Optional: Path to the SQL queries JSON file, defaulting to sql_queries.json if not set. ")
	interval := flag.Int("interval", 0, "Optional: Interval in minutes to run the program repeatedly. Must be greater or equal to 1 minute.")
	duration := flag.Int("duration", 0, "Optional: Duration in hours to keep running the program repeatedly. Must be greater or equal to 1 hour.")

	// Parse the command-line flags
	flag.Parse()

	// Prompt the user to confirm they have reviewed the JSON file
	fmt.Println("=======================================================================================================================================================")
	fmt.Println("                                                                                                                                                       ")
	fmt.Println("IMPORTANT - Please Read !!!")
	fmt.Println("Before proceeding, ensure you have reviewed the JSON file containing the SQL queries to be executed and fully understand the implications of running these queries.")
	fmt.Println("You have confirmed that the SQL queries will not delete data or maliciously alter the database.")
	fmt.Println("Do not execute any SQL queries unless you are certain of their purpose. If you are unsure, review the SQL queries in the JSON file carefully.")
	fmt.Println("Type 'yes' to confirm and proceed, or any other key to exit.")
	fmt.Println("                                                                                                                                                       ")
	fmt.Println("=======================================================================================================================================================")

	var confirmation string
	fmt.Scanln(&confirmation)
	if strings.ToLower(confirmation) != "yes" {
		fmt.Println("Exiting the application. Please review the JSON file for the SQL queries before proceeding.")
		return
	}

	// Execute SQL queries and create Excel file directly

	// Calculate the total number of iterations if interval and duration are provided
	if *interval > 0 && *duration > 0 {

		totalIterations := (*duration * 60) / *interval
		fmt.Printf("Running the program every %d minute(s) for the next %d hour(s) (%d iterations).\n", *interval, *duration, totalIterations)

		for i := 0; i < totalIterations; i++ {
			fmt.Printf("Iteration %d/%d: Executing SQL queries...\n", i+1, totalIterations)
			executeSQLQueriesAndCreateExcel(*sqlConfigProp, *sqlQueries)

			// Wait for the specified interval before the next iteration
			if i < totalIterations-1 {
				time.Sleep(time.Duration(*interval) * time.Minute)
			}
		}

		fmt.Println("Program has completed all iterations. Exiting.")
	} else {
		// Run the program once if no interval or duration is provided
		executeSQLQueriesAndCreateExcel(*sqlConfigProp, *sqlQueries)

	}
}

/*
 * executeSQLQueriesAndCreateExcel reads the SQL Server configuration and queries from the specified files,
 * executes the queries on the database, and writes the results directly to an Excel file without
 * creating intermediate CSV files.
 *
 * Parameters:
 * - sqlConfigProp: A string representing the path to the SQL Server configuration file.
 * - sqlQueries: A string representing the path to the JSON file containing the SQL queries.
 *
 * Functionality:
 * 1. Reads the SQL Server configuration from the `sqlConfigProp` file using the `readSQLConfig` function.
 * 2. Establishes a connection to the SQL Server database using the `connectToDB` function.
 * 3. Reads the SQL queries from the `sqlQueries` file using the `readQueries` function.
 * 4. Creates a new Excel file with a timestamped name.
 * 5. Creates an "executed_queries" sheet as the first sheet with query metadata.
 * 6. Iterates through the queries, executes each query, and writes results directly to separate Excel sheets.
 * 7. Saves the completed Excel file.
 *
 * Notes:
 * - This function eliminates the need for temporary CSV files and directory management.
 * - Each query result is written to a separate sheet in the Excel file.
 * - The first sheet contains metadata about all executed queries.
 * - Memory usage is optimized by processing one query at a time.
 */
func executeSQLQueriesAndCreateExcel(sqlConfigProp string, sqlQueries string) {

	// Read the SQL Server Connection Configuration
	sqlConfig := readSQLConfig(sqlConfigProp)

	db := connectToDB(sqlConfig)
	defer db.Close()

	// Read the JSON file containing the SQL Server Queries to be executed
	queries := readQueries(sqlQueries)

	// Create Excel file with timestamp
	currentTime := time.Now()
	excelFileName := fmt.Sprintf("sql_diagnostics_%s.xlsx", currentTime.Format("02012006_150405"))

	// Check if the Excel file exists and remove it if it does
	if _, err := os.Stat(excelFileName); err == nil {
		if err := os.Remove(excelFileName); err != nil {
			log.Fatalf("Failed to remove existing Excel file: %v", err)
		}
	}

	// Create a new Excel file
	f := excelize.NewFile()

	// Create the executed_queries sheet first
	executedQueriesSheetName := "executed_queries"
	f.SetSheetName("Sheet1", executedQueriesSheetName)

	// Write headers for executed_queries sheet
	f.SetCellValue(executedQueriesSheetName, "A1", "Sr.No")
	f.SetCellValue(executedQueriesSheetName, "B1", "Query")
	f.SetCellValue(executedQueriesSheetName, "C1", "Query Notes")

	// Write query metadata to executed_queries sheet
	for i, query := range queries.Queries {
		rowNum := i + 2 // Start from row 2 (after header)
		f.SetCellValue(executedQueriesSheetName, fmt.Sprintf("A%d", rowNum), i+1)
		f.SetCellValue(executedQueriesSheetName, fmt.Sprintf("B%d", rowNum), query.Query)
		f.SetCellValue(executedQueriesSheetName, fmt.Sprintf("C%d", rowNum), query.Notes)
	}

	// Execute each query and create a sheet for each result
	for i, query := range queries.Queries {
		fmt.Printf("Executing Query: %s\nDescription: %s\n", query.Name, query.Description)
		fmt.Println("Query:", query.Query)

		sheetName := createSheetName(i+1, query.Name)

		// Execute query and write directly to Excel sheet
		err := executeQueryToExcel(db, query.Query, f, sheetName)
		if err != nil {
			log.Printf("Failed to execute query %s: %v", query.Name, err)
			continue
		}
	}

	// Save the Excel file
	if err := f.SaveAs(excelFileName); err != nil {
		log.Fatalf("Error saving Excel file: %v", err)
	}

	fmt.Printf("Excel file created successfully: %s\n", excelFileName)
}

/*
 * readSQLConfig checks for the existence of the SQL configuration file and reads its contents.
 *
 * Parameters:
 * - filePath: A string representing the path to the SQL configuration file.
 *
 * Returns:
 * - SQLServerConfig: A struct containing the SQL Server configuration details, including host, port, database name,
 *   user credentials, and whether to use integrated security (trusted connection).
 *
 * Functionality:
 * 1. Verifies if the specified SQL configuration file exists in the current directory.
 *    - If the file exists, it calls the `getSQLServerConfig` function to read and parse the configuration details.
 *    - If the file does not exist, the function logs an error message and terminates the program.
 * 2. Returns the parsed `SQLServerConfig` struct if the file is successfully read and parsed.
 *
 * Notes:
 * - The function assumes that the configuration file is well-formed and contains all required keys.
 * - If the file does not exist, the program terminates with a fatal error message.
 *
 * Example Usage:
 * config := readSQLConfig("config.properties")
 * fmt.Printf("SQL Server Host: %s\n", config.SQLServerHost)
 */
func readSQLConfig(filePath string) SQLServerConfig {
	if _, err := os.Stat(filePath); err == nil || os.IsExist(err) {
		return getSQLServerConfig(filePath)
	}
	log.Fatalf("Please validate that %s existing in current directory for sql configuration", filePath)
	panic(fmt.Sprintf("Please validate that %s existing in current directory for sql configuration", filePath))
}

/*
 * connectToDB establishes a connection to the SQL Server database using the provided configuration.
 *
 * Parameters:
 * - sqlConfig: A `SQLServerConfig` struct containing the database connection details, such as host, port,
 *   database name, user credentials, and whether to use integrated security (trusted connection).
 *
 * Returns:
 * - *sql.DB: A pointer to the `sql.DB` object representing the database connection.
 *
 * Functionality:
 * 1. Constructs the SQL Server connection string based on the provided configuration.
 *    - If `Trusted` is true, the connection string uses integrated security.
 *    - If `Trusted` is false, the connection string includes the username and password.
 * 2. Opens a connection to the SQL Server database using the constructed connection string.
 * 3. Returns the database connection object (`*sql.DB`) if the connection is successful.
 * 4. Logs a fatal error and terminates the program if the connection fails.
 *
 * Notes:
 * - The function assumes that the `sqlConfig` struct contains valid and complete connection details.
 * - The caller is responsible for closing the database connection when it is no longer needed.
 *
 * Example Usage:
 * sqlConfig := SQLServerConfig{
 *     SQLServerHost: "localhost",
 *     SQLServerPort: "1433",
 *     SQLServerDB:   "TestDB",
 *     SQLServerUser: "sa",
 *     SQLServerPassword: "password",
 *     Trusted:       false,
 * }
 * db := connectToDB(sqlConfig)
 * defer db.Close()
 */
func connectToDB(sqlConfig SQLServerConfig) *sql.DB {
	var slqConnectionString = ""

	// Check if UserDefined connection string is provided and not empty
	if sqlConfig.UserDefined != "" {
		slqConnectionString = sqlConfig.UserDefined

	} else {
		// Construct the connection string based on other fields
		if sqlConfig.Trusted {
			slqConnectionString = "sqlserver://" + sqlConfig.SQLServerHost + ":" + sqlConfig.SQLServerPort + "?database=" + sqlConfig.SQLServerDB + "&connection+timeout=30&trusted_connection=yes&encrypt=false&trustservercertificate=true"
		} else {
			slqConnectionString = "sqlserver://" + sqlConfig.SQLServerUser + ":" + sqlConfig.SQLServerPassword + "@" + sqlConfig.SQLServerHost + ":" + sqlConfig.SQLServerPort + "?database=" + sqlConfig.SQLServerDB + "&connection+timeout=30&encrypt=false&trustservercertificate=true"
		}
	}

	fmt.Printf("Got Connection String %s:\n", slqConnectionString)

	// Open the database connection
	db, err := sql.Open("sqlserver", slqConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Validate the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database, please make sure the connection properties are valid : %v", err)
	}

	return db
}

/*
 * executeQueryToExcel runs a SQL query on the provided database connection and writes the result directly to an Excel sheet.
 *
 * Parameters:
 * - db: A pointer to the `sql.DB` object representing the database connection.
 * - query: A string containing the SQL query to be executed.
 * - f: A pointer to the excelize.File object representing the Excel file.
 * - sheetName: A string representing the name of the Excel sheet where results will be written.
 *
 * Returns:
 * - error: Returns an error if the query execution or Excel writing fails, nil otherwise.
 *
 * Functionality:
 * 1. Executes the provided SQL query using the database connection.
 * 2. Creates a new sheet in the Excel file with the specified name.
 * 3. Writes column headers to the first row of the sheet.
 * 4. Iterates through query results and writes each row to the Excel sheet.
 * 5. Handles different data types appropriately for Excel format.
 *
 * Notes:
 * - The function handles NULL values by converting them to "NULL" strings.
 * - Byte arrays are converted to strings with newlines and carriage returns replaced with spaces.
 * - Memory usage is optimized by processing one row at a time.
 */
func executeQueryToExcel(db *sql.DB, query string, f *excelize.File, sheetName string) error {
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	// Create new sheet
	f.NewSheet(sheetName)

	// Get columns information
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %v", err)
	}

	// Write headers to first row
	for colIndex, colName := range columns {
		cell, _ := excelize.CoordinatesToCellName(colIndex+1, 1)
		f.SetCellValue(sheetName, cell, colName)
	}

	// Create a slice of interface{}'s to hold each column value
	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	// Write data rows
	rowIndex := 2 // Start from row 2 (after headers)
	for rows.Next() {
		err := rows.Scan(values...)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		// Write each cell value
		for colIndex, val := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex)
			v := *(val.(*interface{}))

			if v == nil {
				f.SetCellValue(sheetName, cell, "NULL")
			} else if b, ok := v.([]byte); ok {
				// Handle byte arrays by converting to string and cleaning up
				cleanValue := strings.ReplaceAll(strings.ReplaceAll(string(b), "\n", " "), "\r", " ")
				f.SetCellValue(sheetName, cell, cleanValue)
			} else {
				// Handle other types
				cleanValue := strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " "), "\r", " ")
				f.SetCellValue(sheetName, cell, cleanValue)
			}
		}
		rowIndex++
	}

	// Check for errors during row iteration
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error occurred during row iteration: %v", err)
	}

	return nil
}

/*
 * createSheetName generates a sanitized sheet name for Excel based on the query index and name.
 * Excel has specific restrictions on sheet names (31 character limit, no special characters).
 *
 * Parameters:
 * - index: The index of the query, used as a prefix in the sheet name.
 * - queryName: The name of the query, which will be sanitized and included in the sheet name.
 *
 * Returns:
 * - A string representing the sanitized sheet name in the format "<index>_<sanitized_query_name>".
 *
 * Functionality:
 * 1. Replaces all spaces in the `queryName` with underscores.
 * 2. Removes all special characters from the `queryName` using a regular expression.
 * 3. Concatenates the `index` and the sanitized `queryName`.
 * 4. Truncates the result to 31 characters to comply with Excel sheet name limits.
 * 5. Ensures the sheet name doesn't contain invalid characters for Excel.
 *
 * Example:
 * Input: index = 1, queryName = "Sample Query Name!"
 * Output: "1_Sample_Query_Name"
 *
 * Notes:
 * - Excel sheet names cannot exceed 31 characters.
 * - Excel sheet names cannot contain: \ / ? * [ ] :
 * - The function ensures compliance with these restrictions.
 */
func createSheetName(index int, queryName string) string {
	// Replace spaces with underscores
	queryName = strings.ReplaceAll(queryName, " ", "_")

	// Remove characters that are invalid in Excel sheet names
	// Excel doesn't allow: \ / ? * [ ] :
	re := regexp.MustCompile(`[\\\/\?\*\[\]:]+`)
	queryName = re.ReplaceAllString(queryName, "")

	// Remove other special characters except underscores
	re = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	queryName = re.ReplaceAllString(queryName, "")

	// Concatenate index and sanitized query name
	sheetName := fmt.Sprintf("%d_%s", index, queryName)

	// Excel sheet names cannot exceed 31 characters
	if len(sheetName) > 31 {
		// Keep the index part and truncate the name part
		indexPart := fmt.Sprintf("%d_", index)
		maxNameLength := 31 - len(indexPart)
		if maxNameLength > 0 {
			sheetName = indexPart + queryName[:maxNameLength]
		} else {
			// If index is too long, just use the index
			sheetName = fmt.Sprintf("%d", index)
			if len(sheetName) > 31 {
				sheetName = sheetName[:31]
			}
		}
	}

	return sheetName
}

/*
 * getSQLServerConfig reads the SQL Server configuration from the specified properties file
 * and returns a `SQLServerConfig` struct populated with the configuration details.
 *
 * Parameters:
 * - propFile: A string representing the path to the properties file containing the SQL Server configuration.
 *
 * Returns:
 * - SQLServerConfig: A struct containing the SQL Server configuration details, including host, port, database name,
 *   user credentials, and whether to use integrated security (trusted connection).
 *
 * Functionality:
 * 1. Loads the properties file specified by `propFile` using the `properties` library.
 * 2. Reads the required configuration values (`DB_HOST`, `DB_PORT`, `DB_NAME`, `USER`, `PASSWORD`, `TRUSTED`) from the file.
 * 3. Parses the `TRUSTED` property as a boolean value to determine whether to use integrated security.
 * 4. If any required property is missing, the program terminates with an error.
 * 5. Returns a `SQLServerConfig` struct populated with the configuration values.
 *
 * Notes:
 * - The function assumes that the properties file is well-formed and contains all required keys.
 * - If the `TRUSTED` property is invalid or missing, it defaults to `false`.
 * - The `properties.MustGet` method is used to ensure that required properties are present in the file.
 *
 * Example Usage:
 * config := getSQLServerConfig("config.properties")
 * fmt.Printf("Connecting to SQL Server at %s:%s\n", config.SQLServerHost, config.SQLServerPort)
 */
func getSQLServerConfig(propFile string) SQLServerConfig {

	propertyFile := []string{propFile}

	sqlProperties, error := properties.LoadFiles(propertyFile, properties.UTF8, true)

	if error != nil {
		log.Fatalln("Error Loading Properties File ", error)
		panic(error)
	}

	var sqlServerConfig SQLServerConfig
	sqlServerConfig.UserDefined = sqlProperties.GetString("USER_DEFINED", "")
	sqlServerConfig.UserDefined = strings.TrimSpace(sqlServerConfig.UserDefined)

	if sqlServerConfig.UserDefined == "" {
		sqlServerConfig.SQLServerHost = sqlProperties.MustGet("DB_HOST")
		sqlServerConfig.SQLServerHost = strings.TrimSpace(sqlServerConfig.SQLServerHost)

		sqlServerConfig.SQLServerPort = sqlProperties.MustGet("DB_PORT")
		sqlServerConfig.SQLServerPort = strings.TrimSpace(sqlServerConfig.SQLServerPort)

		sqlServerConfig.SQLServerDB = sqlProperties.MustGet("DB_NAME")
		sqlServerConfig.SQLServerDB = strings.TrimSpace(sqlServerConfig.SQLServerDB)

		sqlServerConfig.SQLServerUser = sqlProperties.MustGet("USER")
		sqlServerConfig.SQLServerUser = strings.TrimSpace(sqlServerConfig.SQLServerUser)

		sqlServerConfig.SQLServerPassword = sqlProperties.MustGet("PASSWORD")
		sqlServerConfig.SQLServerPassword = strings.TrimSpace(sqlServerConfig.SQLServerPassword)

		trusted, err := strconv.ParseBool(sqlProperties.MustGet("TRUSTED"))
		if err != nil {
			fmt.Printf("Invalid Trusted Property: %s, will default to false", sqlProperties.MustGet("TRUSTED"))
			sqlServerConfig.Trusted = false
		} else {
			sqlServerConfig.Trusted = trusted
		}
	}
	return sqlServerConfig
}

/*
 * readQueries reads the SQL queries from a JSON file and returns a Queries object.
 *
 * Parameters:
 * - filePath: A string representing the path to the JSON file containing the SQL queries.
 *
 * Returns:
 * - Queries: A struct containing the parsed SQL queries and their metadata.
 *
 * Functionality:
 * 1. Reads the content of the specified JSON file into memory.
 * 2. Parses the JSON content into a `Queries` struct using the `json.Unmarshal` function.
 * 3. If any errors occur during file reading or JSON parsing, the function logs the error and terminates the program.
 *
 * Notes:
 * - The function assumes that the JSON file is well-formed and adheres to the expected structure.
 * - The `Queries` struct must match the structure of the JSON file for successful parsing.
 *
 * Example Usage:
 * queries := readQueries("sql_queries.json")
 * fmt.Printf("Loaded %d queries from the JSON file.\n", len(queries.Queries))
 */
func readQueries(filePath string) Queries {
	file, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}

	var queries Queries
	err = json.Unmarshal(file, &queries)
	if err != nil {
		log.Fatalf("Failed to parse JSON file: %v", err)
	}

	return queries
}

/*
 * SQLServerConfig holds the configuration details required to connect to a SQL Server database.
 * It includes information such as the host, port, database name, user credentials, and whether
 * to use integrated security (trusted connection).
 *
 * Fields:
 * - UserDefined: User defined DB Connection, this can be any free form format supported by the driver https://github.com/microsoft/go-mssqldb#readme
 * - SQLServerHost: The hostname or IP address of the SQL Server.
 * - SQLServerPort: The port number on which the SQL Server is listening.
 * - SQLServerDB: The name of the database to connect to.
 * - SQLServerUser: The username for authentication (if not using a trusted connection).
 * - SQLServerPassword: The password for authentication (if not using a trusted connection).
 * - Trusted: A boolean indicating whether to use integrated security (trusted connection).
 */
type SQLServerConfig struct {
	UserDefined       string // User defined DB Connection, this can be any free form format supported by the driver https://github.com/microsoft/go-mssqldb#readme
	SQLServerHost     string // Hostname or IP address of the SQL Server
	SQLServerPort     string // Port number on which the SQL Server is listening
	SQLServerDB       string // Name of the database to connect to
	SQLServerUser     string // Username for authentication
	SQLServerPassword string // Password for authentication
	Trusted           bool   // Whether to use integrated security (trusted connection)
}

/*
 * Queries represents a collection of SQL queries along with their metadata.
 * It contains information about the source of the queries and the list of individual queries.
 *
 * Fields:
 * - QuerySource: Metadata about the source of the queries, such as the SQL Server version, author, and other details.
 * - Queries: A list of `Query` objects, each representing a single SQL query with its name, description, and other details.
 */
type Queries struct {
	QuerySource QuerySource `json:"querysource"` // Metadata about the source of the queries
	Queries     []Query     `json:"queries"`     // List of SQL queries
}

/*
 * Query represents a single SQL query along with its metadata.
 * It provides details about the query, including its name, description, and any additional notes.
 *
 * Fields:
 * - Name: The name or identifier of the query.
 * - Description: A brief description of the purpose or functionality of the query.
 * - Query: The actual SQL query string to be executed.
 * - Notes: Additional notes or comments about the query, such as usage instructions or caveats.
 */
type Query struct {
	Name        string `json:"name"`        // Name or identifier of the query
	Description string `json:"description"` // Brief description of the query's purpose
	Query       string `json:"query"`       // The SQL query string
	Notes       string `json:"notes"`       // Additional notes or comments about the query
}

/*
 * QuerySource represents metadata about the source of a collection of SQL queries.
 * It provides information about the SQL Server version, the author, and other details
 * related to the origin and context of the queries.
 *
 * Fields:
 * - SQLServerVersion: Specifies the version of the SQL Server for which the queries are designed.
 * - Name: The name or title of the query source.
 * - Author: The name of the person or entity who authored the queries.
 * - LastModified: The date when the query source was last modified.
 * - Source: A brief description of the source or origin of the queries.
 * - URL: A URL pointing to additional information or documentation about the queries.
 * - Comments: Any additional comments or notes about the query source.
 * - CopyRight: Copyright information related to the query source.
 */
type QuerySource struct {
	SQLServerVersion string `json:"sqlserverversion"` // SQL Server version for which the queries are intended
	Name             string `json:"name"`             // Name or title of the query source
	Author           string `json:"author"`           // Author of the queries
	LastModified     string `json:"lastmodified"`     // Last modification date of the query source
	Source           string `json:"source"`           // Description of the source or origin of the queries
	URL              string `json:"url"`              // URL for additional information or documentation
	Comments         string `json:"comments"`         // Additional comments or notes
	CopyRight        string `json:"copyright"`        // Copyright information
}
