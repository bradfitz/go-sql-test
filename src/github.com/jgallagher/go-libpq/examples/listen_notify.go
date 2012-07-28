package main

import (
	"database/sql"
	"fmt"
	_ "github.com/jgallagher/go-libpq"
	"sync"
)

func pglistener(db *sql.DB, messages chan string, wg *sync.WaitGroup) {
	notifications, err := db.Query("LISTEN mychan")
	if err != nil {
		fmt.Printf("Could not listen to mychan: %s\n", err)
		close(messages)
		return
	}
	defer notifications.Close()

	// tell main() it's okay to spawn the pgnotifier goroutine
	wg.Done()

	var msg string
	for notifications.Next() {
		if err = notifications.Scan(&msg); err != nil {
			fmt.Printf("Error while scanning: %s\n", err)
			continue
		}
		messages <- msg
	}

	fmt.Printf("Lost database connection ?!")
	close(messages)
}

func notifier(db *sql.DB) {
	for i := 0; i < 10; i++ {
		// WARNING: Postgres does not appear to support parameterized notifications
		//          like "NOTIFY mychan, $1". Be careful not to expose SQL injection!
		query := fmt.Sprintf("NOTIFY mychan, 'message-%d'", i)
		if _, err := db.Exec(query); err != nil {
			fmt.Printf("error sending NOTIFY: %s\n", err)
		}
	}
}

func main() {
	db, err := sql.Open("libpq", "") // assuming localhost, user ok, etc
	if err != nil {
		fmt.Printf("could not connect to postgres: %s\n", err)
		return
	}
	defer db.Close()

	messages := make(chan string)
	var wg sync.WaitGroup
	wg.Add(1)
	go pglistener(db, messages, &wg)

	// wait until LISTEN was issued, then spawn notifier goroutine
	wg.Wait()
	go notifier(db)

	for i := 0; i < 10; i++ {
		fmt.Printf("received notification %s\n", <-messages)
	}
}
