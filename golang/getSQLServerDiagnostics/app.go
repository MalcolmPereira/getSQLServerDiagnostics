/*
Package main

This program connects to a SQL Server database, executes a series of SQL queries defined in a JSON file,
and generates diagnostic reports in CSV and Excel formats. It uses a configuration file to define database
connection details and dynamically processes queries to produce results.

Some of the queries use come from https://glennsqlperformance.com/ and we acknowledge this great resource for troubleshooting SQL Server Performance.

Author: Malcolm Pereira
Date: November 27, 2025
Last Modified: November 27, 2025
Revision: 1.0.0

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

- Run the program to generate diagnostic report, that is saved to Excel. The program will first generate CSV file and the combine them into to an excel sheet with all the rows in it.

Dependencies:
	- github.com/denisenkom/go-mssqldb for SQL Server connectivity.
	- github.com/xuri/excelize/v2 for Excel file generation.
	- github.com/magiconair/properties for reading configuration files.

*/

package main

import (
	// Standard library packages
	"encoding/csv"  // For reading and writing CSV files
	"encoding/json" // For parsing and encoding JSON data
	"flag"          // For command line arguments
	"fmt"           // For formatted I/O operations
	"log"           // For logging messages
	"os"            // For interacting with the operating system (e.g., file operations)
	"regexp"        // For working with regular expressions
	"sort"          // For sorting slices and user-defined collections
	"strconv"       // For converting strings to numbers and vice versa
	"strings"       // For string manipulation
	"time"          // For working with date and time

	"database/sql" // Database/sql package for database operations

	// SQL Server driver
	//_ "github.com/denisenkom/go-mssqldb" // Microsoft SQL Server driver for Go From Go Community
	_ "github.com/microsoft/go-mssqldb" // Microsoft SQL Server driver for Go From Microsoft

	// Third-party packages
	"github.com/magiconair/properties" // For reading and handling properties files
	"github.com/xuri/excelize/v2"      // For creating and manipulating Excel files
)

// Default files for config and sql queries
const sql_config = "config.properties" // SQL Server Configuration File
const sql_queries = "sql_queries.json" // SQL Queries File

// Executed Query Details
const executed_queries = "executed_queries.csv"

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
	sqlConfigProp := flag.String("config", sql_config, "Path to the SQL Server configuration file, defaulting to config.properties if not set.")
	sqlQueries := flag.String("queries", sql_queries, "Path to the SQL queries JSON file, defaulting to sql_queries.json if not set. ")

	// Parse the command-line flags
	flag.Parse()

	log.Println("Starting Application...")

	// Create a unique temporary directory for storing CSV files
	tempDir, err := os.MkdirTemp("", "sql_diagnostics_")
	if err != nil {
		log.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Ensure the temporary directory is deleted

	executeSQLQueries(*sqlConfigProp, *sqlQueries, tempDir)

	createExcelFromCSV(tempDir)

	log.Println("Done Application...")
}

/*
 * executeSQLQueries reads the SQL Server configuration and queries from the specified files,
 * executes the queries on the database, and writes the results to CSV files.
 *
 * Parameters:
 * - sqlConfigProp: A string representing the path to the SQL Server configuration file.
 * - sqlQueries: A string representing the path to the JSON file containing the SQL queries.
 *
 * Functionality:
 * 1. Reads the SQL Server configuration from the `sqlConfigProp` file using the `readSQLConfig` function.
 * 2. Deletes the `executed_queries.csv` file if it already exists.
 * 3. Establishes a connection to the SQL Server database using the `connectToDB` function.
 * 4. Creates a new `executed_queries.csv` file to log the executed queries and their metadata.
 * 5. Reads the SQL queries from the `sqlQueries` file using the `readQueries` function.
 * 6. Iterates through the queries, executes each query using the `executeQuery` function, and writes the results to individual CSV files.
 * 7. Logs any errors encountered during file operations or query execution.
 *
 * Notes:
 * - The function assumes that the configuration and query files are well-formed and accessible.
 * - The database connection is closed automatically after all queries are executed.
 * - Each query result is saved to a separate CSV file, with the file name generated dynamically using the `createFileName` function.
 *
 * Example Usage:
 * executeSQLQueries("config.properties", "sql_queries.json")
 */
func executeSQLQueries(sqlConfigProp string, sqlQueries string, tempDir string) {

	//Read the SQL Server Connection Configuration
	sqlConfig := readSQLConfig(sqlConfigProp)

	db := connectToDB(sqlConfig)
	defer db.Close()

	//Read the JSON file containing the SQL Server Queries to be executed
	fileCounter := 1
	queries := readQueries(sqlQueries)

	// Change the current working directory to the temporary directory
	originalDir, err := os.Getwd() // Save the original working directory
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalDir) // Ensure we return to the original directory after execution

	err = os.Chdir(tempDir) // Change to the temporary directory
	if err != nil {
		log.Fatalf("Failed to change to temporary directory: %v", err)
	}

	// Check if the CSV file exists and remove it if it does
	if _, err := os.Stat(executed_queries); err == nil {
		if err := os.Remove(executed_queries); err != nil {
			log.Fatalf("Failed to remove existing executed_queries.csv file: %v", err)
		}
	}

	//Create CSV file
	csvFile, err := os.Create(executed_queries)
	if err != nil {
		log.Fatalf("Failed to create executed_queries.csv file: %v", err)
	}
	defer csvFile.Close()

	//Get Writer
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write CSV Header Row
	err = writer.Write([]string{"Sr.No", "Query", "Query Notes"})
	if err != nil {
		log.Printf("Failed to write query to CSV file: %v", err)
	}

	for i, query := range queries.Queries {

		// Write query details to CSV
		err = writer.Write([]string{strconv.Itoa(i + 1), query.Query, query.Notes})
		if err != nil {
			log.Printf("Failed to write query to CSV file: %v", err)
		}

		fmt.Printf("Executing Query: %s\nDescription: %s\n", query.Name, query.Description)
		fmt.Println("Query:", query.Query)

		fileName := createFileName(fileCounter, query.Name)

		// Check if the CSV file exists and remove it if it does
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				log.Fatalf("Failed to remove existing %s file: %v", fileName, err)
			}
		}

		executeQuery(db, query.Query, fileName)

		fileCounter++
	}
}

/*
 * createExcelFromCSV generates an Excel file from multiple CSV files in the current directory.
 * It combines the data from all CSV files into separate sheets in the Excel file, with the
 * `executed_queries.csv` file always appearing as the first sheet. The function also deletes
 * the original CSV files after successfully creating the Excel file.
 *
 * Parameters:
 * - excelFileName: The name of the Excel file to be created.
 *
 * Functionality:
 * 1. Checks if the specified Excel file already exists and deletes it if it does.
 * 2. Reads all files in the current directory and filters out only the `.csv` files.
 * 3. Separates `executed_queries.csv` from other CSV files to ensure it appears as the first sheet.
 * 4. Sorts the remaining CSV files based on their numeric prefixes for consistent ordering.
 * 5. Creates a new Excel file and adds each CSV file's data to a separate sheet.
 *    - The sheet name is derived from the CSV file name (excluding the `.csv` extension).
 * 6. Saves the Excel file with the specified name.
 * 7. Deletes all the original CSV files after the Excel file is successfully created.
 *
 * Notes:
 * - The function uses the `excelize` library to create and manipulate Excel files.
 * - If any errors occur while reading CSV files or saving the Excel file, the function logs the error
 *   and exits gracefully.
 * - The function assumes that the CSV files are well-formed and contain valid data.
 *
 * Example Usage:
 * createExcelFromCSV("sql_diagnostics_27112025_143045.xlsx")
 */
func createExcelFromCSV(tempDir string) {

	// Change the current working directory to the temporary directory
	originalDir, err := os.Getwd() // Save the original working directory
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalDir) // Ensure we return to the original directory after execution

	err = os.Chdir(tempDir) // Change to the temporary directory
	if err != nil {
		log.Fatalf("Failed to change to temporary directory: %v", err)
	}

	// Combine all CSV files in the temp directory into an Excel file
	currentTime := time.Now()
	excelFileName := fmt.Sprintf("sql_diagnostics_%s.xlsx", currentTime.Format("02012006_150405"))

	// Check if the CSV file exists and remove it if it does
	if _, err := os.Stat(excelFileName); err == nil {
		if err := os.Remove(excelFileName); err != nil {
			log.Fatalf("Failed to remove existing excelFileName file: %v", err)
		}
	}

	// Get all files in the current directory
	files, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	// Filter CSV files
	var csvFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
			csvFiles = append(csvFiles, file.Name())
		}
	}

	// Separate executed_queries.csv
	var otherFiles []string
	for _, file := range csvFiles {
		if file != "executed_queries.csv" {
			otherFiles = append(otherFiles, file)
		}
	}

	// Custom sort for other files
	sort.SliceStable(otherFiles, func(i, j int) bool {
		// Extract numeric prefixes for comparison
		iPrefix, iErr := strconv.Atoi(strings.SplitN(otherFiles[i], "_", 2)[0])
		jPrefix, jErr := strconv.Atoi(strings.SplitN(otherFiles[j], "_", 2)[0])

		if iErr == nil && jErr == nil {
			return iPrefix < jPrefix
		}
		return otherFiles[i] < otherFiles[j]
	})

	// Combine executed_queries.csv with sorted files
	sortedFiles := append([]string{"executed_queries.csv"}, otherFiles...)

	// Create a new Excel file
	f := excelize.NewFile()

	for i, csvFile := range sortedFiles {

		// Open the CSV file
		file, err := os.Open(csvFile)
		if err != nil {
			fmt.Printf("Error opening file %s: %v\n", csvFile, err)
			continue
		}

		// Read the CSV file
		reader := csv.NewReader(file)
		rows, err := reader.ReadAll()
		file.Close() // Ensure the file is closed immediately after reading
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", csvFile, err)
			continue
		}

		// Create a new sheet with the name of the CSV file (without extension)
		sheetName := strings.TrimSuffix(csvFile, ".csv")
		fmt.Printf("Now Adding Sheet Name %s \n", sheetName)
		if i == 0 {
			f.SetSheetName("Sheet1", sheetName)
		} else {
			f.NewSheet(sheetName)
		}

		// Write rows to the sheet
		for rowIndex, row := range rows {
			for colIndex, cellValue := range row {
				cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				f.SetCellValue(sheetName, cell, cellValue)
			}
		}
	}

	// Delete all files in sortedFiles
	for _, csvFileDelete := range sortedFiles {
		err := os.Remove(csvFileDelete)
		if err != nil {
			fmt.Printf("Error deleting file %s: %v\n", csvFileDelete, err)
		} else {
			fmt.Printf("Deleted file: %s\n", csvFileDelete)
		}
	}

	err = os.Chdir(originalDir) // Change to the original working directory
	if err != nil {
		log.Fatalf("Failed to change to working directory: %v", err)
	}

	// Save the Excel file
	if err := f.SaveAs(excelFileName); err != nil {
		fmt.Printf("Error saving Excel file: %v\n", err)
		return
	}

	// Debug: Print final sheet count
	//sheets := f.GetSheetList()
	//fmt.Printf("Total sheets in Excel file: %d\n", len(sheets))
	//for _, s := range sheets {
	//	fmt.Printf("Sheet in Excel: %s\n", s)
	//}

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
	if sqlConfig.Trusted {
		slqConnectionString = "sqlserver://" + sqlConfig.SQLServerHost + ":" + sqlConfig.SQLServerPort + "?database=" + sqlConfig.SQLServerDB + "&connection+timeout=30&trusted_connection=yes&encrypt=false&trustservercertificate=true"
	} else {
		slqConnectionString = "sqlserver://" + sqlConfig.SQLServerUser + ":" + sqlConfig.SQLServerPassword + "@" + sqlConfig.SQLServerHost + ":" + sqlConfig.SQLServerPort + "?database=" + sqlConfig.SQLServerDB + "&connection+timeout=30&encrypt=false&trustservercertificate=true"
	}
	db, err := sql.Open("sqlserver", slqConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	return db
}

/*
 * executeQuery runs a SQL query on the provided database connection and writes the result to a CSV file.
 *
 * Parameters:
 * - db: A pointer to the `sql.DB` object representing the database connection.
 * - query: A string containing the SQL query to be executed.
 * - fileName: A string representing the name of the CSV file where the query results will be saved.
 *
 * Functionality:
 * 1. Executes the provided SQL query using the database connection.
 * 2. Fetches the query results and writes them to the specified CSV file.
 * 3. Logs any errors encountered during query execution or file writing.
 *
 * Notes:
 * - The function assumes that the database connection (`db`) is valid and open.
 * - If the query fails or the file cannot be written, the function logs the error and terminates the program.
 * - The CSV file will contain the query results, with the first row being the column headers.
 *
 * Example Usage:
 * db, err := sql.Open("mssql", connectionString)
 * if err != nil {
 *     log.Fatalf("Failed to connect to database: %v", err)
 * }
 * defer db.Close()
 *
 * executeQuery(db, "SELECT * FROM Users", "users.csv")
 */
func executeQuery(db *sql.DB, query string, fileName string) {
	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	defer rows.Close() // Ensure rows are closed to release resources

	// Open the CSV file for writing
	csvFile, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to create fileName CSV file %s: %v", fileName, err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Handle rows if the query returns results
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("Failed to get columns: %v", err)
		return
	}

	// Write the header row to the CSV file
	err = writer.Write(columns)
	if err != nil {
		log.Printf("Failed to write header to CSV file: %v", err)
		return
	}

	// Create a slice of interface{}'s to hold each column value
	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	// Iterate through the rows
	for rows.Next() {
		err := rows.Scan(values...)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		// Convert the row values to strings for CSV writing
		row := make([]string, len(columns))
		for i, val := range values {
			v := *(val.(*interface{}))
			if v == nil {
				row[i] = "NULL"
			} else if b, ok := v.([]byte); ok {
				row[i] = strings.ReplaceAll(strings.ReplaceAll(string(b), "\n", " "), "\r", " ")
			} else {
				row[i] = strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " "), "\r", " ")
			}
		}

		// Write the row to the CSV file
		err = writer.Write(row)
		if err != nil {
			log.Printf("Failed to write row to CSV file: %v", err)
		}
	}

	// Check for errors during row iteration
	if err = rows.Err(); err != nil {
		log.Printf("Error occurred during row iteration: %v", err)
	}
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
	sqlServerConfig.SQLServerHost = sqlProperties.MustGet("DB_HOST")
	sqlServerConfig.SQLServerPort = sqlProperties.MustGet("DB_PORT")
	sqlServerConfig.SQLServerDB = sqlProperties.MustGet("DB_NAME")
	sqlServerConfig.SQLServerUser = sqlProperties.MustGet("USER")
	sqlServerConfig.SQLServerPassword = sqlProperties.MustGet("PASSWORD")
	trusted, err := strconv.ParseBool(sqlProperties.MustGet("TRUSTED"))
	if err != nil {
		fmt.Printf("Invalid Trusted Property: %s, will default to false", sqlProperties.MustGet("TRUSTED"))
		sqlServerConfig.Trusted = false
	} else {
		sqlServerConfig.Trusted = trusted
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
 * createFileName generates a sanitized file name for a query result based on the query index and name.
 * The function ensures that the file name is safe for use in the file system by replacing spaces with
 * underscores and removing special characters.
 *
 * Parameters:
 * - index: The index of the query, used as a prefix in the file name.
 * - queryName: The name of the query, which will be sanitized and included in the file name.
 *
 * Returns:
 * - A string representing the sanitized file name in the format "<index>_<sanitized_query_name>.csv".
 *
 * Functionality:
 * 1. Replaces all spaces in the `queryName` with underscores.
 * 2. Removes all special characters from the `queryName` using a regular expression.
 * 3. Concatenates the `index` and the sanitized `queryName` to generate the file name.
 * 4. Appends the `.csv` extension to the file name.
 *
 * Example:
 * Input: index = 1, queryName = "Sample Query Name!"
 * Output: "1_Sample_Query_Name.csv"
 *
 * Notes:
 * - The function ensures that the generated file name is valid and safe for use in most file systems.
 * - Special characters such as `!`, `@`, `#`, etc., are removed to avoid issues with file system compatibility.
 */
func createFileName(index int, queryName string) string {
	// Replace spaces with underscores
	queryName = strings.ReplaceAll(queryName, " ", "_")

	// Remove special characters using regex
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	queryName = re.ReplaceAllString(queryName, "")

	// Concatenate index and sanitized query name
	return fmt.Sprintf("%d_%s.csv", index, queryName)
}

/*
 * SQLServerConfig holds the configuration details required to connect to a SQL Server database.
 * It includes information such as the host, port, database name, user credentials, and whether
 * to use integrated security (trusted connection).
 *
 * Fields:
 * - SQLServerHost: The hostname or IP address of the SQL Server.
 * - SQLServerPort: The port number on which the SQL Server is listening.
 * - SQLServerDB: The name of the database to connect to.
 * - SQLServerUser: The username for authentication (if not using a trusted connection).
 * - SQLServerPassword: The password for authentication (if not using a trusted connection).
 * - Trusted: A boolean indicating whether to use integrated security (trusted connection).
 */
type SQLServerConfig struct {
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
