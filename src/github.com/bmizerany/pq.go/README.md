# pq.go - A pure Go Postgres driver (works with exp/sql)

## Connecting
		import (
			"exp/sql"
			_ "github.com/bmizerany/pq.go"
		)

		db, err := sql.Open("postgres", "postgres://blake:@locahost:5432")
		if err != nil {
			log.Print(err)
		}


## Unnamed Prepeared Query

		rows, err := db.Query("SELECT length($1) AS foo", "hello")
		if err != nil {
			log.Print(err)
		}

		var length int
		for rows.Next() {
			err := rows.Scan(&length)
			if err != nil {
				log.Print(err)
				break
			}

			log.Printf("length = %d", length)
		}

		if rows.Error() != nil {
			log.Print(rows.Error())
			break
		}

## Notifications

Notifications can't be accessed via the exp/sql package. You will need to use
pq.OpenRaw to obtain a single connection for listenting.  NOTE: It is recommend
to only use this connection for reading notifiactions and to use the exp/sql
API for all other operations. This may change in the future.

**Example**

		db, err := sql.Open("postgres", "postgres://blake:@localhost:5432/mydb")
		if err != nil {
			panic(err)
		}

		ln, err = pq.OpenRaw("postgres://blake:@localhost:5432/mydb")
		if err != nil {
			panic(err)
		}

		// Concurrently read notifications to avoid blocking the connection (see To Know).
		go func() {
			for n := range ln.Notifies {
				log.Printf("notify: %q:%q", n.Channel, n.Payload)
			}
		}()

		ln.Exec("LISTEN user_added")
		db.Exec("INSERT INTO user (first, last) VALUES ($1, $2)", "Blake", "Mizerany")
		db.Exec("SELECT pg_notify(user_added, $1 || " " || $2)", "Blake", "Mizerany")

**To Know**

When one or more LISTEN's are active, it is the responsiblity of the user to
drain the `db.Notifies` channel; Failing to do so causes reads on the
connection to block if there are pending notifications on the connection.
