# TODO

## 1. Bestanden in de prompt en placeholders invullen

Wens van de gebruiker (2026-11-26): als `how` een commando met een placeholder voorstelt (bijv. `tail -f <file>`), wil je alleen die waarde snel invullen voordat het commando draait. Nu gaat zo'n commando ongewijzigd naar de shell en faalt het (`<file>` wordt als redirect gelezen).

- [ ] `internal/suggest/suggest.go`: voeg de inhoud van de huidige map toe aan de systeemprompt, zodat het model zelf het juiste bestand kan kiezen. Maximaal 50 items, niet recursief, mappen met een `/` erachter, verborgen bestanden overslaan behalve nuttige zoals `.env`, en "… and N more" als de lijst is afgekapt. Als de map niet te lezen is, laat je de lijst zonder foutmelding weg.
- [ ] Herken placeholders in het voorgestelde commando met `<[a-zA-Z][\w-]*>`. Echte shell-syntax zoals `<<EOF`, `<(cmd)`, `2>&1` en `< input.txt` telt niet mee.
- [ ] `internal/ui/ask.go`: heeft het commando placeholders, dan gaat enter meteen naar een invulformulier. Elke unieke placeholder krijgt één tekstveld met de naam als label. Enter gaat naar het volgende veld, en na het laatste draait het ingevulde commando.
- [ ] Kopiëren (`c`) gaat ook via het formulier, zodat een commando nooit met placeholders erin gekopieerd of uitgevoerd wordt.
- [ ] Komt een placeholder vaker voor (bijv. `cp <file> <file>.bak`), dan vraag je hem één keer en vul je hem overal in.
- [ ] Een ingevulde waarde krijgt alleen enkele quotes als er spaties of speciale shell-tekens in zitten (`tail -f app.log` blijft zonder quotes).
- [x] Besloten (2026-11-26): de printmodus (`-p`, of als de uitvoer gepipet wordt) blijft ongewijzigd.
- [ ] Tests voor het herkennen van placeholders, het samenvoegen van dubbele placeholders, het quoten en de bestandslijst. Draai `how` daarna zelf in een terminal met een vraag die een placeholder oplevert, en controleer het formulier, de run en de copy.

## 2. Bestandskiezer voor bestands-placeholders

Wens van de gebruiker (2026-11-26): bij een placeholder voor een bestand wil je een bestand uit de huidige map kunnen kiezen in plaats van het te typen.

- [ ] Placeholders met `file`, `path`, `dir`, `log`, `src` of `dest` in de naam tonen onder het tekstveld de items van de huidige map, gefilterd terwijl je typt. Kies er een met de pijltjestoetsen en enter.
- [ ] Je kunt altijd een eigen pad typen, ook als het niet in de lijst staat. Andere placeholders houden een gewoon tekstveld.
- [ ] Tests voor het herkennen van bestands-placeholders en het filteren. Draai `how` daarna zelf in een terminal en controleer of kiezen en vrij typen allebei werken.

## Backlog
