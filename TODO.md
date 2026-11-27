# TODO

## 1. Bestanden in de prompt en placeholders invullen

Wens van de gebruiker (2026-11-26): als `how` een commando met een placeholder voorstelt (bijv. `tail -f <file>`), wil je alleen die waarde snel invullen voordat het commando draait. Nu gaat zo'n commando ongewijzigd naar de shell en faalt het (`<file>` wordt als redirect gelezen).

- [x] `internal/suggest/suggest.go`: voeg de inhoud van de huidige map toe aan de systeemprompt, zodat het model zelf het juiste bestand kan kiezen. Maximaal 50 items, niet recursief, mappen met een `/` erachter, verborgen bestanden overslaan behalve nuttige zoals `.env`, en "… and N more" als de lijst is afgekapt. Als de map niet te lezen is, laat je de lijst zonder foutmelding weg.
- [x] Herken placeholders in het voorgestelde commando met `<[a-zA-Z][\w-]*>`. Echte shell-syntax zoals `<<EOF`, `<(cmd)`, `2>&1` en `< input.txt` telt niet mee.
- [x] `internal/ui/ask.go`: heeft het commando placeholders, dan gaat enter meteen naar een invulformulier. Elke unieke placeholder krijgt één tekstveld met de naam als label. Enter gaat naar het volgende veld, en na het laatste draait het ingevulde commando.
- [x] Kopiëren (`c`) gaat ook via het formulier, zodat een commando nooit met placeholders erin gekopieerd of uitgevoerd wordt.
- [x] Komt een placeholder vaker voor (bijv. `cp <file> <file>.bak`), dan vraag je hem één keer en vul je hem overal in.
- [x] Een ingevulde waarde krijgt alleen enkele quotes als er spaties of speciale shell-tekens in zitten (`tail -f app.log` blijft zonder quotes).
- [x] Besloten (2026-11-26): de printmodus (`-p`, of als de uitvoer gepipet wordt) blijft ongewijzigd.
- [x] Tests voor het herkennen van placeholders, het samenvoegen van dubbele placeholders, het quoten en de bestandslijst. Draai `how` daarna zelf in een terminal met een vraag die een placeholder oplevert, en controleer het formulier, de run en de copy.

## 2. Bestandskiezer voor bestands-placeholders

Wens van de gebruiker (2026-11-26): bij een placeholder voor een bestand wil je een bestand uit de huidige map kunnen kiezen in plaats van het te typen.

- [x] Placeholders met `file`, `path`, `dir`, `log`, `src` of `dest` in de naam tonen onder het tekstveld de items van de huidige map, gefilterd terwijl je typt. Kies er een met de pijltjestoetsen en enter.
- [x] Je kunt altijd een eigen pad typen, ook als het niet in de lijst staat. Andere placeholders houden een gewoon tekstveld.
- [x] Tests voor het herkennen van bestands-placeholders en het filteren. Draai `how` daarna zelf in een terminal en controleer of kiezen en vrij typen allebei werken.

## 3. Vervolgvraag op een suggestie

Wens van de gebruiker (2026-11-27): na een antwoord wil je er snel iets aan toevoegen zonder de hele vraag opnieuw te typen. Bijvoorbeeld "how do i export this wordpress db" geeft een `mysqldump`-commando, en met de vervolgvraag "with the wp cli" krijg je `wp db export`.

- [x] `internal/ui/ask.go`: in de suggestieweergave opent `f` een invoerregel voor een vervolgvraag. Enter stuurt hem naar het model en toont de spinner, esc gaat terug naar de suggestie. Een lege vervolgvraag doet niets.
- [x] `internal/suggest/suggest.go`: `Ask` krijgt de gespreksgeschiedenis mee: na de few-shot voorbeelden komen alle eerdere vragen (`user`) en antwoorden (`assistant`, als dezelfde JSON die het model teruggaf), dan de nieuwe vervolgvraag. Er kunnen meerdere vervolgvragen achter elkaar komen; de hele geschiedenis gaat steeds mee.
- [x] Voeg aan de few-shot voorbeelden één vervolgvraag toe (vraag, antwoord, vervolgvraag, aangepast antwoord), zodat kleine modellen zien dat ze op het vorige commando moeten voortbouwen.
- [x] Besloten (2026-11-27): er is geen aparte modus voor "vervangen" of "erachter plakken". Het model kiest dat zelf op basis van de vervolgvraag; de bestaande regel om stappen met `&&` te combineren dekt het erachter plakken.
- [x] Besloten (2026-11-27): de vervolgvraag bouwt voort op het commando dat in de weergave staat. (Een bewerking met `e` wordt nu bij esc weggegooid en bij enter meteen uitgevoerd, dus die hoeft niet mee.)
- [x] Besloten (2026-11-27): `r` stelt na een vervolgvraag de laatste vraag opnieuw, met dezelfde geschiedenis ervoor.
- [x] Besloten (2026-11-27): de hulpregel wordt `enter run · e edit · c copy · f follow-up · r retry · q quit`. De printmodus (`-p`, of als de uitvoer gepipet wordt) blijft ongewijzigd. Werk de README bij met de nieuwe toets.
- [x] Tests voor het opbouwen van de berichtenlijst met geschiedenis (volgorde en rollen). Draai `how` daarna zelf in een terminal: stel een vraag, geef een of twee vervolgvragen en controleer dat het nieuwe commando op het vorige voortbouwt, en dat run, copy en retry daarna nog werken.

## Backlog
