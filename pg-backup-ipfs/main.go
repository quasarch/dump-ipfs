package main

import (
	"context"
	"fmt"
	pg "github.com/habx/pg-commands"
	"github.com/ipfs/go-cid"
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
	connStrs := os.Args[1:]

	client, err := w3s.NewClient(w3s.WithToken(os.Getenv("API_KEY")))
	if err != nil {
		panic(err)
	}

	c := cron.New(cron.WithSeconds())
	_, err = c.AddFunc("*/2 * * * * *", func() {
		for _, conn := range connStrs {
			result, err := dumpDB(conn)
			if err != nil {
				log.Println(err)
			}
			c, err := putFileToIPFS(client, result)
			if err != nil {
				log.Println(err)
			}
			fmt.Printf("https://ipfs.io/ipfs/%s\n", c)
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

func putFileToIPFS(client w3s.Client, filename string) (cid.Cid, error) {
	// Store in Filecoin with a timestamp.
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

func newFilename(dbName string) string {
	return fmt.Sprintf("%v_%v.sql", dbName, time.Now().Unix())
}

func dumpDB(connStr string) (string, error) {
	db, err := connectDB(connStr)
	if err != nil {
		return "", err
	}
	dump, err := pg.NewDump(db)
	dump.Format = new(string)
	*dump.Format = "p"
	dump.SetFileName(newFilename(db.DB))
	if err != nil {
		return "", err
	}
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
