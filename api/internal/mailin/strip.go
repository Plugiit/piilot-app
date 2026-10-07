package mailin

import (
	"html"
	"regexp"
	"strings"
)

// Ce qu'une reponse par e-mail traine derriere elle : la citation du message
// precedent, la signature, « Envoye de mon iPhone ». Le ticket n'en garde que
// ce que la personne vient d'ecrire.

var (
	// « Le lun. 7 oct. 2026 a 10:02, Piilot <support@…> a ecrit : »
	attributionFR = regexp.MustCompile(`(?i)^\s*le\s.{4,250}\s?a\s+[ée]crit\s*:?\s*$`)
	// « On Mon, Oct 7, 2026 at 10:02 AM Piilot <support@…> wrote: »
	attributionEN = regexp.MustCompile(`(?i)^\s*on\s.{4,250}\s?wrote\s*:?\s*$`)
	// Separateurs d'Outlook et des clients plus anciens.
	originalMessage = regexp.MustCompile(`(?i)^\s*-{2,}\s*(original message|message d'origine|message original|message transf[ée]r[ée]|forwarded message)\s*-{2,}`)
	underscores     = regexp.MustCompile(`^\s*_{10,}\s*$`)
	// L'en-tete d'Outlook : « De : … » suivi d'« Envoye : » ou « Objet : ».
	outlookFrom   = regexp.MustCompile(`(?i)^\s*\*?(de|from)\s*:\*?\s+\S`)
	outlookFields = regexp.MustCompile(`(?i)^\s*\*?(envoy[ée]|sent|date|[àa]|to|objet|subject|cc)\s*:\*?\s`)
	// Les signatures de messagerie mobile.
	sentFrom = regexp.MustCompile(`(?i)^\s*(envoy[ée] (de|depuis) mon|sent from my|get outlook for|t[ée]l[ée]chargez outlook)`)
)

// StripReply rend ce que la personne vient d'ecrire, sans la citation du
// message auquel elle repond, ni sa signature. Si tout est coupe — un e-mail
// qui n'est qu'une citation transferee —, le texte entier est rendu : mieux
// vaut trop que rien.
func StripReply(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")

	cut := len(lines)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		joined := line
		// L'attribution est souvent coupee en deux lignes par le client.
		if i+1 < len(lines) {
			joined = strings.TrimRight(line, " ") + " " + strings.TrimSpace(lines[i+1])
		}

		switch {
		case attributionFR.MatchString(line), attributionEN.MatchString(line):
			cut = i
		case (attributionFR.MatchString(joined) || attributionEN.MatchString(joined)) &&
			!strings.HasPrefix(strings.TrimSpace(line), ">"):
			cut = i
		case originalMessage.MatchString(line), underscores.MatchString(line):
			cut = i
		case outlookFrom.MatchString(line) && outlookBlock(lines[i+1:]):
			cut = i
		case strings.TrimRight(line, " ") == "--" && i > 0:
			// Le separateur de signature normalise.
			cut = i
		case strings.HasPrefix(strings.TrimSpace(line), ">") && quoteRunsToEnd(lines[i:]):
			cut = i
		}
		if cut != len(lines) {
			break
		}
	}

	kept := make([]string, 0, cut)
	for _, line := range lines[:cut] {
		if sentFrom.MatchString(line) {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
	}

	out := strings.TrimSpace(collapseBlankLines(strings.Join(kept, "\n")))
	if out == "" {
		return strings.TrimSpace(collapseBlankLines(text))
	}
	return out
}

// outlookBlock : un « De : » ouvre un en-tete de message cite quand l'une des
// quatre lignes suivantes porte un autre champ d'en-tete.
func outlookBlock(next []string) bool {
	for i := 0; i < len(next) && i < 4; i++ {
		if outlookFields.MatchString(next[i]) {
			return true
		}
	}
	return false
}

// quoteRunsToEnd : une citation « > » qui ne laisse ensuite que des lignes
// citees ou vides. Une citation au milieu d'une reponse — on cite une phrase
// pour y repondre — reste a sa place.
func quoteRunsToEnd(lines []string) bool {
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, ">") {
			return false
		}
	}
	return true
}

func collapseBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

var (
	// Ou commence la citation dans un e-mail HTML : Gmail, Apple Mail,
	// Outlook, Thunderbird.
	htmlQuoteStart = regexp.MustCompile(`(?is)<div[^>]+class="[^"]*gmail_quote|<blockquote[^>]+type="cite"|<div[^>]+id="(divRplyFwdMsg|appendonsend)"|<div[^>]+class="[^"]*moz-cite-prefix|<hr[^>]+id="stopSpelling"`)
	htmlDrop       = regexp.MustCompile(`(?is)<(head|style|script|title)[^>]*>.*?</(head|style|script|title)>`)
	htmlBreak      = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/tr|/h[1-6]|/blockquote)\s*/?>`)
	htmlItem       = regexp.MustCompile(`(?i)<\s*li[^>]*>`)
	htmlTag        = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlSpaces     = regexp.MustCompile(`[ \t\x{a0}]+`)
)

// HTMLToText rend le texte d'un e-mail HTML, sans la citation du message
// precedent quand la messagerie la balise.
func HTMLToText(body string) string {
	if loc := htmlQuoteStart.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	body = htmlDrop.ReplaceAllString(body, "")
	body = htmlItem.ReplaceAllString(body, "\n- ")
	body = htmlBreak.ReplaceAllString(body, "\n")
	body = htmlTag.ReplaceAllString(body, "")
	body = html.UnescapeString(body)

	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(htmlSpaces.ReplaceAllString(l, " "))
	}
	return strings.TrimSpace(collapseBlankLines(strings.Join(lines, "\n")))
}

// Body rend ce que la personne a ecrit : le texte s'il y en a un, le HTML
// converti sinon, sans citation ni signature.
func (m Message) Body() string {
	if strings.TrimSpace(m.Text) != "" {
		return StripReply(m.Text)
	}
	if strings.TrimSpace(m.HTML) != "" {
		return StripReply(HTMLToText(m.HTML))
	}
	return ""
}

// FullBody rend tout le texte, citations comprises : pour l'ecran de tri, ou
// l'on veut voir l'e-mail tel qu'il est arrive.
func (m Message) FullBody() string {
	if strings.TrimSpace(m.Text) != "" {
		return strings.TrimSpace(strings.ReplaceAll(m.Text, "\r\n", "\n"))
	}
	// Sans couper la citation : le HTML est converti en entier.
	body := htmlDrop.ReplaceAllString(m.HTML, "")
	body = htmlItem.ReplaceAllString(body, "\n- ")
	body = htmlBreak.ReplaceAllString(body, "\n")
	body = html.UnescapeString(htmlTag.ReplaceAllString(body, ""))
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(htmlSpaces.ReplaceAllString(l, " "))
	}
	return strings.TrimSpace(collapseBlankLines(strings.Join(lines, "\n")))
}

var subjectPrefix = regexp.MustCompile(`(?i)^\s*((re|tr|fw|fwd|aw|wg|r|rif|sv|vs)\s*(\[\d+\])?\s*:\s*)+`)

// CleanSubject retire les « Re : », « TR : », « Fwd: » d'un objet.
func CleanSubject(subject string) string {
	return strings.TrimSpace(subjectPrefix.ReplaceAllString(subject, ""))
}
