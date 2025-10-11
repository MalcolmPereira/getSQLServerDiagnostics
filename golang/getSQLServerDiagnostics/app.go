// START: TODO WORK
// TODO: Add description for this module and how to use it
// END: TODO WORK

//config.propertiesfile that drives the database connection must be of the following format
/*
	DB_HOST=<host name>
	DB_PORT=<port>
	DB_NAME=<datbase name>
	USER=<user>
	PASSWORD=<password
	TRUSTED=<use integrated security true or false in which case USER and PASSWORD is not needed>
*/

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"database/sql"

	_ "github.com/denisenkom/go-mssqldb"

	"github.com/magiconair/properties"
)

//START: TODO WORK
//TODO: Remove hardcoded file names and make it dynamic so these  can be passed from the command line or config file

// SQL Server Configuration File
const sqlConfigProp = "config.properties"

// SQL Queries File
const sqlquerries = "sql_queries.json"

// Executed Query Details
const executed_queries = "executed_queries.csv"

//END: TODO WORK

func main() {
	log.Println("Starting Application...")
	sqlConfig := readSQLConfig(sqlConfigProp)

	// Check if the CSV file exists and remove it if it does
	if _, err := os.Stat(executed_queries); err == nil {
		if err := os.Remove(executed_queries); err != nil {
			log.Fatalf("Failed to remove existing executed_queries.csv file: %v", err)
		}
	}

	db := connectToDB(sqlConfig)
	defer db.Close()

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

	queries := readQueries(sqlquerries)
	for i, query := range queries.Queries {

		// Write query details to CSV
		err = writer.Write([]string{strconv.Itoa(i + 1), query.Query, query.Notes})
		if err != nil {
			log.Printf("Failed to write query to CSV file: %v", err)
		}

		fmt.Printf("Executing Query: %s\nDescription: %s\n", query.Name, query.Description)
		fmt.Println("Query:", query.Query)

		fileName := createFileName(i, query.Name)

		// Check if the CSV file exists and remove it if it does
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				log.Fatalf("Failed to remove existing %s file: %v", fileName, err)
			}
		}

		executeQuery(db, query.Query, fileName)
	}
	log.Println("Done Application...")
}

/**
 * connectToDB establishes a connection to the SQL Server database using the provided configuration.
 * @param config The SQLServerConfig containing the database connection details.
 * @return *sql.DB The database connection object.
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

/**
 * executeQuery runs a simple query and prints the result.
 * @param db The database connection object.
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
			row[i] = strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", *(val.(*interface{}))), "\n", " "), "\r", " ")
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

/**
 * readSQLConfig checks for the existence of the SQL configuration file and reads its contents.
 * @param filePath The path to the SQL configuration file.
 * @return SQLServerConfig The SQL Server configuration details.
 */
func readSQLConfig(filePath string) SQLServerConfig {
	if _, err := os.Stat(filePath); err == nil || os.IsExist(err) {
		return getSQLServerConfig(filePath)
	}
	log.Fatalf("Please validate that %s existing in current directory for sql configuration", sqlConfigProp)
	panic(fmt.Sprintf("Please validate that %s existing in current directory for sql configuration", sqlConfigProp))
}

/**
 * getSQLServerConfig reads the SQL Server configuration from the specified properties file.
 * @param propFile The path to the properties file.
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

/**
 * readQueries reads the SQL queries from the JSON file and returns a Queries object.
 * @param filePath The path to the JSON file.
 * @return Queries The parsed queries.
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

/* createFileName generates a sanitized file name based on the query index and name.
 * It replaces spaces with underscores and removes special characters.
 * @param index The index of the query.
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
 * SQLServerConfig holds the configuration details for connecting to a SQL Server database.
 */
type SQLServerConfig struct {
	SQLServerHost     string
	SQLServerPort     string
	SQLServerDB       string
	SQLServerUser     string
	SQLServerPassword string
	Trusted           bool
}

/**
 * Queries holds a collection of Query objects.
 */
type Queries struct {
	QuerySource QuerySource `json:"querysource"`
	Queries     []Query     `json:"queries"`
}

/*
* Query represents a single SQL query with its name and description.
 */
type Query struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Query       string `json:"query"`
	Notes       string `json:"notes"`
}

/*
* Query represents a single SQL query with its name and description.
 */
type QuerySource struct {
	SQLServerVersion string `json:"sqlserververion"`
	Name             string `json:"name"`
	Author           string `json:"author"`
	LastModield      string `json:"lastmodified"`
	Source           string `json:"source"`
	URL              string `json:"url"`
	Comments         string `json:"comments"`
	CopyRight        string `json:"copyright"`
}
