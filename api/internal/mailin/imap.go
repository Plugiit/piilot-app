package mailin

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// IMAP est un client IMAP4rev1 reduit a ce que la releve demande : se
// connecter, ouvrir un dossier, lister les messages non lus, en lire un,
// le marquer lu.
//
// Ecrit ici plutot qu'importe : six commandes, sans etat a tenir entre deux
// releves. Un client complet apporterait l'IDLE, les extensions, le
// multiplexage — rien dont une releve chaque minute ait besoin.
type IMAP struct {
	conn net.Conn
	r    *bufio.Reader
	tag  int
}

// IMAPConfig decrit une boite.
type IMAPConfig struct {
	Host     string
	Port     int
	TLS      bool
	Username string
	Password string
	Folder   string
	Timeout  time.Duration
}

// DialIMAP ouvre la connexion et s'authentifie.
func DialIMAP(cfg IMAPConfig) (*IMAP, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: cfg.Timeout}

	var conn net.Conn
	var err error
	if cfg.TLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("connexion a %s : %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(cfg.Timeout * 4))

	c := &IMAP{conn: conn, r: bufio.NewReader(conn)}

	// Le salut du serveur.
	greeting, err := c.readLine()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("salut du serveur : %w", err)
	}
	if !strings.HasPrefix(greeting, "* OK") && !strings.HasPrefix(greeting, "* PREAUTH") {
		conn.Close()
		return nil, fmt.Errorf("serveur IMAP : %s", greeting)
	}

	if _, err := c.command("LOGIN", astring(cfg.Username), astring(cfg.Password)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("authentification refusee : %w", err)
	}
	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	if _, err := c.command("SELECT", astring(folder)); err != nil {
		c.Close()
		return nil, fmt.Errorf("dossier %s : %w", folder, err)
	}

	return c, nil
}

// Unseen rend les UID des messages non lus, du plus ancien au plus recent.
func (c *IMAP) Unseen() ([]uint32, error) {
	lines, err := c.command("UID SEARCH UNSEEN")
	if err != nil {
		return nil, err
	}
	var uids []uint32
	for _, l := range lines {
		if !strings.HasPrefix(l.text, "* SEARCH") {
			continue
		}
		for _, f := range strings.Fields(strings.TrimPrefix(l.text, "* SEARCH")) {
			if n, err := strconv.ParseUint(f, 10, 32); err == nil {
				uids = append(uids, uint32(n))
			}
		}
	}
	return uids, nil
}

// Fetch lit un message entier sans le marquer lu (BODY.PEEK) : il ne l'est
// qu'une fois depose en base.
func (c *IMAP) Fetch(uid uint32) ([]byte, error) {
	lines, err := c.command(fmt.Sprintf("UID FETCH %d (BODY.PEEK[])", uid))
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		if strings.HasPrefix(l.text, "* ") && strings.Contains(strings.ToUpper(l.text), "FETCH") && l.literal != nil {
			return l.literal, nil
		}
	}
	return nil, fmt.Errorf("message %d introuvable", uid)
}

// MarkSeen marque un message lu.
func (c *IMAP) MarkSeen(uid uint32) error {
	_, err := c.command(fmt.Sprintf(`UID STORE %d +FLAGS.SILENT (\Seen)`, uid))
	return err
}

// Close se deconnecte poliment.
func (c *IMAP) Close() error {
	_, _ = c.command("LOGOUT")
	return c.conn.Close()
}

// line est une ligne de reponse, et le litteral qu'elle annonce le cas
// echeant.
type line struct {
	text    string
	literal []byte
}

// command envoie une commande et lit jusqu'a sa reponse etiquetee.
func (c *IMAP) command(name string, args ...arg) ([]line, error) {
	c.tag++
	tag := fmt.Sprintf("p%d", c.tag)

	// Les arguments litteraux ({n}) attendent l'invitation « + » du serveur
	// avant d'envoyer leurs octets.
	var buf bytes.Buffer
	buf.WriteString(tag + " " + name)
	for _, a := range args {
		buf.WriteByte(' ')
		if !a.literal {
			buf.WriteString(a.text)
			continue
		}
		fmt.Fprintf(&buf, "{%d}\r\n", len(a.text))
		if _, err := c.conn.Write(buf.Bytes()); err != nil {
			return nil, err
		}
		buf.Reset()
		cont, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(cont, "+") {
			return nil, fmt.Errorf("litteral refuse : %s", cont)
		}
		buf.WriteString(a.text)
	}
	buf.WriteString("\r\n")
	if _, err := c.conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	var out []line
	for {
		l, err := c.readResponse()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(l.text, tag+" ") {
			status := strings.TrimPrefix(l.text, tag+" ")
			if strings.HasPrefix(status, "OK") {
				return out, nil
			}
			return nil, errors.New(status)
		}
		out = append(out, l)
	}
}

// readResponse lit une ligne de reponse, avec ses litteraux : « {123} » en fin
// de ligne annonce 123 octets bruts, apres lesquels la ligne continue.
func (c *IMAP) readResponse() (line, error) {
	var text strings.Builder
	var literal []byte
	for {
		l, err := c.readLine()
		if err != nil {
			return line{}, err
		}
		text.WriteString(l)
		n, ok := literalSize(l)
		if !ok {
			break
		}
		if n > MaxRawBytes {
			return line{}, ErrTooLarge
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(c.r, data); err != nil {
			return line{}, err
		}
		if literal == nil {
			literal = data
		}
	}
	return line{text: text.String(), literal: literal}, nil
}

func literalSize(l string) (int, bool) {
	if !strings.HasSuffix(l, "}") {
		return 0, false
	}
	open := strings.LastIndexByte(l, '{')
	if open < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(l[open+1:len(l)-1], "+"))
	if err != nil {
		return 0, false
	}
	return n, true
}

func (c *IMAP) readLine() (string, error) {
	s, err := c.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

// arg est un argument de commande : chaine entre guillemets, ou litteral
// quand elle ne s'ecrit pas entre guillemets.
type arg struct {
	text    string
	literal bool
}

func astring(s string) arg {
	for _, r := range s {
		if r > 0x7e || r < 0x20 {
			return arg{text: s, literal: true}
		}
	}
	return arg{text: `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`}
}
