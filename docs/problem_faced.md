# The Great Database Disconnect: A Debugging Adventure

Welcome, junior adventurer! Today, we’re going to look back at a treacherous quest we just completed. Our mission was simple: get our Go API (`trigger-api`) to talk to our PostgreSQL database running inside a Docker container. 

But alas, we were met with two sneaky villains that blocked our path. Here is the tale of what they were, why they caused chaos, and how we ultimately defeated them.

---

## Villain 1: The Misguided Map (Connection String Mismatch)

### What is it?
In the world of backend development, a **Connection String** (often called a DSN - Data Source Name) is exactly like a treasure map. It contains the exact coordinates (host and port) and the secret passphrase (username and password) that your application needs to find and unlock the database.

### What problem did this cause?
Our API was given a faulty map. Inside our `main.go` file, the default connection string told the app to look for the database on **port 5432** and use the password **`postgres`**. 

However, our actual database was set up via `docker-compose.yml` to listen on **port 5440** (on our host machine) and expected the password **`postgres_password`**. 

Because our API knocked on the wrong door with the wrong password, the database security guards threw a `FATAL: password authentication failed` error and kicked us out!

### How we solved it:
We simply updated the map! We went into `main.go` and changed the fallback connection string to match the reality of our Docker setup:
`postgres://postgres:postgres_password@localhost:5440/trigger_db?sslmode=disable`

---

## Villain 2: The Invisible Stowaways (CRLF vs. LF Line Endings)

### What is it?
When you press "Enter" on your keyboard to create a new line, your computer secretly inserts invisible characters to mark the end of that line. 
* **Windows** inserts two characters: a Carriage Return (`\r`) and a Line Feed (`\n`). This is known as **CRLF**.
* **Unix/Linux** (which is what runs inside Docker containers) only uses one character: a Line Feed (`\n`). This is known as **LF**.

### What problem did this cause?
We had a script named `init-db.sh` that was supposed to run automatically when the database first started up to create our `trigger_db`. Because we were working on a Windows machine, the script was saved with **CRLF** line endings. 

When the Linux-based Docker container tried to run this script, it got confused by the invisible `\r` characters. It saw `\r` as a weird, garbage command and threw a `/bin/bash^M: bad interpreter` error. 

Because the script crashed immediately, our database (`trigger_db`) was never actually created. When our API finally got the right password (from Villain 1) and knocked on the door, the database said: `FATAL: database "trigger_db" does not exist`.

### How we solved it:
We had to exorcise the invisible stowaways! We ran a command to strip all the `\r` characters out of `init-db.sh`, converting the file to purely use Unix (LF) line endings. 

Since Docker only runs initialization scripts on a completely fresh database, we then had to wipe the slate clean. We deleted the old, broken database storage volume (`docker compose down -v`) and spun everything back up (`docker compose up -d`). This time, the script ran flawlessly, the database was created, and our API connected successfully!

---

**Adventure Summary:** Always double-check your connection credentials against your infrastructure config, and never underestimate the chaos that invisible line-ending characters can cause when crossing between Windows and Linux environments!
