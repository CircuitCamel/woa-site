# War of Ages

**War of Ages** is my Dungeons & Dragons campaign.  
This repository contains the source code and content for the website I built to organize the campaign for my players.

The site includes character pages, session logs, and custom rules.

Also, the site can be viewed on [dnd.circuitcamel.com](https://dnd.circuitcamel.com) for the main branch.

## Features

- Character bios
- Session stories
- Homebrew rules
- Markdown-based content
- Go web server
- Simple CSS styling
- Login-only character sheets for players, with an admin dashboard

## Run Locally

```bash
make full
```

## Character Sheets

Players log in at `/sheets` to build and run their characters. There is no sign-up: the DM adds players from the dashboard at `/admin`.

On first run the server creates an admin account from `ADMIN_USERNAME` and `ADMIN_PASSWORD`. If no password is set, it prints a random one to the console once. Everyone has to replace their temporary password when they first log in.

Data is stored in a SQLite database at `DB_PATH` (`./data/warofages.db` by default), so back that file up. The Go server does all the rules, dice, and saving. The JavaScript in `static/scripts` runs only in the browser, and the sheets still work without it.

Rules content comes from the SRD 5.1 through the open [5e-database](https://github.com/5e-bits/5e-database) project (MIT). Run `make srd` to rebuild it.

This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC, available at [dnd.wizards.com](https://dnd.wizards.com/resources/systems-reference-document). The SRD 5.1 is licensed under the [Creative Commons Attribution 4.0 International License](https://creativecommons.org/licenses/by/4.0/legalcode).


---
This project is licensed under MIT.
