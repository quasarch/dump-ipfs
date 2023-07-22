package main

import (
	"context"
	"database/sql"
	"fmt"
	pg "github.com/habx/pg-commands"
	"github.com/ipfs/go-cid"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"github.com/web3-storage/go-w3s-client"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type PostgresConfig struct {
	Scheme   string
	Username string
	Password string
	Host     string
	Port     int64
	DB       string
}

func main() {
	// postgresql://username:password@localhost:5432/database_name
	cronStr := os.Args[1]
	connStrs := os.Args[2:]

	client, err := w3s.NewClient(w3s.WithToken(os.Getenv("API_KEY")))
	if err != nil {
		panic(err)
	}

	dbs := make([]*sql.DB, 0)
	for _, conn := range connStrs {
		db, err := sql.Open("postgres", fmt.Sprintf("%s?sslmode=disable", conn))
		if err != nil {
			log.Printf("Error opening database connection: %v", err)
			continue
		}
		dbs = append(dbs, db)
	}

	c := cron.New(cron.WithSeconds())
	_, err = c.AddFunc(cronStr, func() {
		for i, conn := range connStrs {
			currTimestamp := currentTimestamp()
			result, err := dumpDB(conn, currTimestamp)
			if err != nil {
				log.Println(err)
			}
			c, err := putFileToIPFS(client, result)
			if err != nil {
				log.Println(err)
			}
			fmt.Printf("https://ipfs.io/ipfs/%s\n", c)
			// add a new row to the dump database
			// row: , filename, ipfs-url
			insertDumpRow(dbs[i], currTimestamp, result, fmt.Sprintf("https://ipfs.io/ipfs/%s", c))
		}
	})
	if err != nil {
		panic(err)
	}

	cError := make(chan error, 1)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		c.Run()
		defer func() {
			if rec := recover(); rec != nil {
				cError <- fmt.Errorf("running cron: %v", rec)
			}
		}()
	}()

	select {
	case <-shutdown:
		c.Stop()
	case err := <-cError:
		log.Fatal(err)
	}
}

func createTable(db *sql.DB) {
	// , filename, ipfs-url
	query := `
		CREATE TABLE IF NOT EXISTS ipfs_dumps (
			timestamp INTEGER PRIMARY KEY,
			filename TEXT NOT NULL,
			ipfs_url TEXT NOT NULL
		)
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Error creating table: %v", err)
	}

}

func insertDumpRow(db *sql.DB, timestamp int, filename string, ipfsURL string) {
	// ensure table exists
	createTable(db)
	// insert a new row into the table
	query := `
		INSERT INTO ipfs_dumps (timestamp, filename, ipfs_url)
		VALUES ($1, $2, $3)
	`

	_, err := db.Exec(query, timestamp, filename, ipfsURL)
	if err != nil {
		log.Printf("Error inserting row: %v", err)
	}
}

func putFileToIPFS(client w3s.Client, filename string) (cid.Cid, error) {
	// Store in Filecoin with a .
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	c, err := client.Put(ctx, file)
	if err != nil {
		panic(err)
	}

	return c, nil
}

func parseConnStr(connStr string) (*PostgresConfig, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, err
	}
	password, _ := u.User.Password()
	port, err := strconv.ParseInt(u.Port(), 10, 64)
	if err != nil {
		return nil, err
	}

	return &PostgresConfig{
		Scheme:   u.Scheme,
		Username: u.User.Username(),
		Password: password,
		Host:     u.Hostname(),
		Port:     port,
		DB:       u.Path[1:],
	}, nil
}

func connectDB(connStr string) (*pg.Postgres, error) {
	dbConfig, err := parseConnStr(connStr)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Connecting to %s:%d/%s\n", dbConfig.Host, dbConfig.Port, dbConfig.DB)

	return &pg.Postgres{
		Host:     dbConfig.Host,
		Port:     int(dbConfig.Port),
		DB:       dbConfig.DB,
		Username: dbConfig.Username,
		Password: dbConfig.Password,
	}, nil
}

func currentTimestamp() int {
	return int(time.Now().Unix())
}

func newFilename(dbName string, timestamp int) string {
	return fmt.Sprintf("%v_%v.sql", dbName, timestamp)
}

func dumpDB(connStr string, timestamp int) (string, error) {
	db, err := connectDB(connStr)
	if err != nil {
		return "", err
	}
	dump, err := pg.NewDump(db)
	if err != nil {
		return "", err
	}
	dump.SetFileName(newFilename(db.DB, timestamp))
	dump.SetupFormat("p")
	dump.EnableVerbose()

	dumpExec := dump.Exec(pg.ExecOptions{StreamPrint: false})
	if dumpExec.Error != nil {
		log.Println(dumpExec.Error.Err)
		log.Println(dumpExec.Output)

	} else {
		log.Println("Dump success")
		log.Println(dumpExec.Output)
	}

	return dumpExec.File, nil
}
