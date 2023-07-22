package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	pg "github.com/habx/pg-commands"
	"github.com/ipfs/go-cid"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"github.com/web3-storage/go-w3s-client"
	"io"
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
	encryptionKey := []byte(os.Getenv("ENCRYPTION_KEY"))

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
			c, err := putFileToIPFS(client, result, encryptionKey)
			if err != nil {
				log.Println(err)
			}
			fmt.Printf("https://ipfs.io/ipfs/%s\n", c)
			// add a new row to the dump database
			// row: , filename, ipfs-url
			insertDumpRow(dbs[i], currTimestamp, result, fmt.Sprintf("https://ipfs.io/ipfs/%s", c), GetMD5Hash(encryptionKey))
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
		CREATE SCHEMA IF NOT EXISTS dump_ipfs;

		CREATE TABLE IF NOT EXISTS dump_ipfs.backup_log (
			timestamp INTEGER PRIMARY KEY,
			filename TEXT NOT NULL,
			ipfs_url TEXT NOT NULL,
			key_checksum TEXT NOT NULL
		)
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Error creating table: %v", err)
	}

}

func insertDumpRow(db *sql.DB, timestamp int, filename, ipfsURL, keyChecksum string) {
	// ensure table exists
	createTable(db)
	// insert a new row into the table
	query := `
		INSERT INTO dump_ipfs.backup_log (timestamp, filename, ipfs_url, key_checksum)
		VALUES ($1, $2, $3, $4)
	`

	_, err := db.Exec(query, timestamp, filename, ipfsURL, keyChecksum)
	if err != nil {
		log.Printf("Error inserting row: %v", err)
	}
}

func putFileToIPFS(client w3s.Client, filename string, encryptionKey []byte) (cid.Cid, error) {
	// Store in Filecoin with a .
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("reading file: %w", err)
	}

	encryptedData, err := Encrypt(encryptionKey, data)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("encrypting file: %w", err)
	}

	if err := os.WriteFile(file.Name()+".enc", encryptedData, 0777); err != nil {
		return cid.Cid{}, fmt.Errorf("writting encrypted file: %w", err)
	}

	file, err = os.Open(file.Name() + ".enc")
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

func GetMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func Encrypt(key, data []byte) ([]byte, error) {
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}

	// NOTE: For extra security we could store the nonce in the dump_ipfs.backup_log table to not expose it to IPFS.
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}
