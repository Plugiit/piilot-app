//go:build integration

// Tests d'integration du bot Discord : cles machine, liaison par code et flux
// d'evenements.
//
// Ils figent ce que le code seul ne montre pas : un code ne sert qu'une fois,
// un evenement s'ecrit dans la transaction du changement — ou pas du tout —,
// et un compte desactive disparait pour le bot sans perdre sa liaison.
package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const testDiscordSecret = "secret-de-test-suffisamment-long-32"

func newDiscord(t *testing.T) (*usecase.DiscordService, *usecase.BotEventService, *pgxpool.Pool) {
	t.Helper()
	_, pool := newService(t)
	return usecase.NewDiscordService(pool, []byte(testDiscordSecret)), usecase.NewBotEventService(pool), pool
}

// snowflake tire un identifiant Discord de 18 chiffres, propre au test.
func snowflake() string {
	return strconv.FormatInt(100000000000000000+rand.Int64N(800000000000000000), 10)
}

// headCursor rend le dernier evenement emis : les assertions lisent ce qui suit.
func headCursor(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT last_id FROM bot_event_cursor`).Scan(&id); err != nil {
		t.Fatalf("lecture du compteur : %v", err)
	}
	return id
}

type seenEvent struct {
	Type          string
	DiscordUserID string
	Role          string
}

// eventsSince relit le flux apres un curseur, en ne gardant que les membres
// donnes : d'autres tests ecrivent dans le meme flux.
func eventsSince(t *testing.T, events *usecase.BotEventService, after int64, members ...string) []seenEvent {
	t.Helper()
	keep := map[string]bool{}
	for _, m := range members {
		keep[m] = true
	}
	var out []seenEvent
	for {
		page, err := events.Events(context.Background(), &after, 100)
		if err != nil {
			t.Fatalf("lecture du flux : %v", err)
		}
		for _, e := range page.Items {
			var data struct {
				DiscordUserID string `json:"discord_user_id"`
				Role          string `json:"role"`
			}
			if err := json.Unmarshal(e.Data, &data); err != nil {
				t.Fatalf("donnees de l'evenement %s : %v", e.ID, err)
			}
			if keep[data.DiscordUserID] {
				out = append(out, seenEvent{Type: e.Type, DiscordUserID: data.DiscordUserID, Role: data.Role})
			}
		}
		next, err := strconv.ParseInt(page.NextAfter, 10, 64)
		if err != nil {
			t.Fatalf("next_after illisible : %q", page.NextAfter)
		}
		if len(page.Items) == 0 {
			return out
		}
		after = next
	}
}

func wantEvents(t *testing.T, got []seenEvent, want ...seenEvent) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("evenements = %+v\n attendus   %+v", got, want)
	}
}

// link lie un compte a un membre, de bout en bout : code tire, puis rachete.
func link(t *testing.T, discord *usecase.DiscordService, userID uuid.UUID, member string) usecase.DiscordLinkCreated {
	t.Helper()
	ctx := context.Background()
	code, err := discord.CreateCode(ctx, userID)
	if err != nil {
		t.Fatalf("tirage du code : %v", err)
	}
	out, err := discord.Redeem(ctx, code.Code, member)
	if err != nil {
		t.Fatalf("rachat du code : %v", err)
	}
	return out
}

// --- Cles du bot -------------------------------------------------------------

func TestUneCleDuBotSeVerifieJusquASaRevocation(t *testing.T) {
	_, pool := newService(t)
	keys := usecase.NewBotKeyService(pool)
	ctx := context.Background()

	name := "test-" + uuid.NewString()[:8]
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM bot_keys WHERE name = $1`, name) })

	raw, key, err := keys.Create(ctx, name)
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	if key.Role != "bot" {
		t.Errorf("role = %q, attendu bot", key.Role)
	}

	var stored []byte
	if err := pool.QueryRow(ctx, `SELECT key_hash FROM bot_keys WHERE id = $1`, key.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == raw {
		t.Fatal("la cle est stockee en clair")
	}

	role, id, ok, err := keys.AuthenticateBotKey(ctx, raw)
	if err != nil || !ok || role != "bot" || id != key.ID {
		t.Fatalf("cle valide : role=%q id=%v ok=%v err=%v", role, id, ok, err)
	}
	var lastUsed *string
	if err := pool.QueryRow(ctx, `SELECT last_used_at::text FROM bot_keys WHERE id = $1`, key.ID).Scan(&lastUsed); err != nil || lastUsed == nil {
		t.Errorf("last_used_at non date apres usage : %v", err)
	}

	// Un second nom actif identique est refuse ; une seconde cle sous un autre
	// nom est permise, pour tourner sans coupure.
	if _, _, err := keys.Create(ctx, name); !errors.Is(err, usecase.ErrBotKeyNameTaken) {
		t.Fatalf("nom en double : %v, attendu ErrBotKeyNameTaken", err)
	}
	if _, _, err := keys.Create(ctx, "Nom Invalide"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("nom invalide : %v", err)
	}

	if _, err := keys.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("revocation : %v", err)
	}
	if _, _, ok, err := keys.AuthenticateBotKey(ctx, raw); ok || err != nil {
		t.Fatalf("cle revoquee encore acceptee : ok=%v err=%v", ok, err)
	}
	if _, err := keys.Revoke(ctx, key.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("seconde revocation : %v", err)
	}

	var audits int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE target_type = 'bot_key' AND target_id = $1 AND action IN ('bot_key.created', 'bot_key.revoked')`,
		key.ID.String()).Scan(&audits); err != nil || audits != 2 {
		t.Errorf("journal d'audit : %d lignes, attendu 2 (%v)", audits, err)
	}

	// Le nom se libere avec la revocation.
	if _, _, err := keys.Create(ctx, name); err != nil {
		t.Fatalf("nom d'une cle revoquee : %v", err)
	}
	if _, _, ok, _ := keys.AuthenticateBotKey(ctx, "pas-une-cle"); ok {
		t.Fatal("une chaine quelconque est acceptee")
	}
}

func TestLeRoleBotNePeutEtrePorteParUnCompte(t *testing.T) {
	_, pool := newService(t)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (email, password_hash, firstname, lastname, role) VALUES ($1, 'x', 'B', 'O', 'bot')`,
		"bot-"+uuid.NewString()+"@piilot.test")
	if err == nil {
		t.Fatal("un compte a recu le role bot")
	}
}

func TestLeRoleBotResteHorsDeLEcranDesRoles(t *testing.T) {
	accounts, _, _ := newAccounts(t)
	ctx := context.Background()

	matrix, err := accounts.Roles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range matrix.Roles {
		if r.Code == "bot" {
			t.Error("le role bot parait dans l'ecran des roles")
		}
		for _, p := range r.Grantable {
			if p == "discord.redeem" || p == "discord.read" || p == "events.read" {
				t.Errorf("%s peut recevoir %s", r.Code, p)
			}
		}
	}
	if _, err := accounts.SetRolePermissions(ctx, "bot", nil); !errors.Is(err, usecase.ErrRoleLocked) {
		t.Fatalf("permissions du bot modifiees : %v", err)
	}
	if _, err := accounts.SetRolePermissions(ctx, "team", []string{"events.read"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("events.read accordee a team : %v", err)
	}
}

// --- Liaison par code ----------------------------------------------------------

func TestUnCodeLieLesComptesEtNeSertQuUneFois(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "client")
	member := snowflake()
	start := headCursor(t, pool)

	code, err := discord.CreateCode(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(code.Code) != 9 || code.Code[4] != '-' {
		t.Fatalf("code %q : XXXX-XXXX attendu", code.Code)
	}

	// Saisie en minuscules et sans tiret : le meme code.
	lower := []byte(code.Code[:4] + code.Code[5:])
	for i, c := range lower {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + 'a' - 'A'
		}
	}
	out, err := discord.Redeem(ctx, string(lower), member)
	if err != nil {
		t.Fatalf("rachat : %v", err)
	}
	if out.DiscordUserID != member || out.Role != "client" || out.ReplacedDiscordUserID != nil || out.UserID != userID {
		t.Fatalf("liaison = %+v", out)
	}

	if _, err := discord.Redeem(ctx, code.Code, snowflake()); !errors.Is(err, usecase.ErrLinkCodeInvalid) {
		t.Fatalf("second rachat : %v, attendu ErrLinkCodeInvalid", err)
	}

	status, err := discord.Status(ctx, userID)
	if err != nil || !status.Linked || status.DiscordUserID == nil || *status.DiscordUserID != member || status.LinkedAt == nil {
		t.Fatalf("statut = %+v, %v", status, err)
	}
	got, err := discord.Link(ctx, member)
	if err != nil || got.Role != "client" {
		t.Fatalf("lecture par le bot : %+v, %v", got, err)
	}

	wantEvents(t, eventsSince(t, events, start, member),
		seenEvent{Type: usecase.BotEventDiscordLinkUpserted, DiscordUserID: member, Role: "client"})
}

func TestUnCodeExpireOuRemplaceNeLiePlus(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")

	first, err := discord.CreateCode(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := discord.CreateCode(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := discord.Redeem(ctx, first.Code, snowflake()); !errors.Is(err, usecase.ErrLinkCodeInvalid) {
		t.Fatalf("code remplace : %v, attendu ErrLinkCodeInvalid", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE discord_link_codes SET expires_at = now() - interval '1 second' WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := discord.Redeem(ctx, second.Code, snowflake()); !errors.Is(err, usecase.ErrLinkCodeInvalid) {
		t.Fatalf("code expire : %v, attendu ErrLinkCodeInvalid", err)
	}

	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM discord_link_codes WHERE user_id = $1`, userID).Scan(&left); err != nil || left != 0 {
		t.Errorf("le code expire n'a pas ete consomme : %d ligne(s)", left)
	}
	if status, _ := discord.Status(ctx, userID); status.Linked {
		t.Error("un code expire a lie le compte")
	}
}

func TestUnCompteDiscordDejaLieAilleursEstRefuseEtLeCodeConsomme(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	alice, _ := createUser(t, pool, "client")
	bob, _ := createUser(t, pool, "client")
	member := snowflake()
	link(t, discord, alice, member)
	start := headCursor(t, pool)

	code, err := discord.CreateCode(ctx, bob)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := discord.Redeem(ctx, code.Code, member); !errors.Is(err, usecase.ErrDiscordAlreadyLinked) {
		t.Fatalf("rachat : %v, attendu ErrDiscordAlreadyLinked", err)
	}

	// Le code est consomme : delier Alice ne le rend pas valable.
	if err := discord.Unlink(ctx, alice); err != nil {
		t.Fatal(err)
	}
	if _, err := discord.Redeem(ctx, code.Code, member); !errors.Is(err, usecase.ErrLinkCodeInvalid) {
		t.Fatalf("code apres un 409 : %v, attendu ErrLinkCodeInvalid", err)
	}

	// Seule la deliaison d'Alice a parle au bot.
	wantEvents(t, eventsSince(t, events, start, member),
		seenEvent{Type: usecase.BotEventDiscordLinkRemoved, DiscordUserID: member})
}

func TestUneNouvelleLiaisonRemplaceLAncienne(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	before, after := snowflake(), snowflake()
	link(t, discord, userID, before)
	start := headCursor(t, pool)

	out := link(t, discord, userID, after)
	if out.ReplacedDiscordUserID == nil || *out.ReplacedDiscordUserID != before {
		t.Fatalf("remplace = %v, attendu %s", out.ReplacedDiscordUserID, before)
	}
	if _, err := discord.Link(ctx, before); !errors.Is(err, usecase.ErrDiscordLinkNotFound) {
		t.Fatalf("l'ancien membre est encore lie : %v", err)
	}

	wantEvents(t, eventsSince(t, events, start, before, after),
		seenEvent{Type: usecase.BotEventDiscordLinkRemoved, DiscordUserID: before},
		seenEvent{Type: usecase.BotEventDiscordLinkUpserted, DiscordUserID: after, Role: "team"})
}

func TestRelierLeMemeMembreNeChangeRien(t *testing.T) {
	discord, events, pool := newDiscord(t)
	userID, _ := createUser(t, pool, "team")
	member := snowflake()
	first := link(t, discord, userID, member)
	start := headCursor(t, pool)

	again := link(t, discord, userID, member)
	if again.ReplacedDiscordUserID != nil || !again.LinkedAt.Equal(first.LinkedAt) {
		t.Fatalf("liaison = %+v, attendu inchangee", again)
	}
	wantEvents(t, eventsSince(t, events, start, member))
}

func TestUnMembreLieAUnCompteDesactiveSeRelieAilleurs(t *testing.T) {
	discord, _, pool := newDiscord(t)
	accounts, _, _ := newAccounts(t)
	ctx := context.Background()
	admin, _ := createUser(t, pool, "admin")
	old, _ := createUser(t, pool, "team")
	fresh, _ := createUser(t, pool, "team")
	member := snowflake()
	link(t, discord, old, member)

	if err := accounts.Disable(ctx, admin, old); err != nil {
		t.Fatal(err)
	}
	out := link(t, discord, fresh, member)
	if out.UserID != fresh {
		t.Fatalf("liaison = %+v", out)
	}
	if status, _ := discord.Status(ctx, old); status.Linked {
		t.Error("l'ancien compte garde la liaison reprise")
	}
}

// redeemTogether rachete deux codes pour le meme membre, en meme temps.
func redeemTogether(discord *usecase.DiscordService, member string, codes ...string) []error {
	errs := make([]error, len(codes))
	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = discord.Redeem(context.Background(), code, member)
		}()
	}
	wg.Wait()
	return errs
}

// Deux comptes rachetent le meme membre en meme temps, l'un deja lie a un
// autre membre : sans verrou sur le membre, l'emission du remplacement et
// l'index unique se croisaient et Postgres tuait une des deux transactions.
func TestDeuxRachatsConcurrentsDuMemeMembreSeSuivent(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()

	for round := range 10 {
		member := snowflake()
		free, _ := createUser(t, pool, "team")
		linked, _ := createUser(t, pool, "team")
		link(t, discord, linked, snowflake())
		first, err := discord.CreateCode(ctx, free)
		if err != nil {
			t.Fatal(err)
		}
		second, err := discord.CreateCode(ctx, linked)
		if err != nil {
			t.Fatal(err)
		}

		ok, conflicts := 0, 0
		for _, err := range redeemTogether(discord, member, first.Code, second.Code) {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, usecase.ErrDiscordAlreadyLinked):
				conflicts++
			default:
				t.Fatalf("tour %d : %v", round, err)
			}
		}
		if ok != 1 || conflicts != 1 {
			t.Fatalf("tour %d : %d liaison(s) et %d conflit(s), attendu 1 et 1", round, ok, conflicts)
		}
	}
}

// Le membre est lie a un compte, qui le rachete encore pendant qu'un autre
// compte tente de le prendre : le second verrouillait le compte du premier
// avant que le premier n'atteigne le membre.
func TestUnRachatConcurrentDuMembreDejaLieNeBloquePas(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()

	for round := range 10 {
		member := snowflake()
		owner, _ := createUser(t, pool, "team")
		other, _ := createUser(t, pool, "team")
		link(t, discord, owner, member)
		ownerCode, err := discord.CreateCode(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		otherCode, err := discord.CreateCode(ctx, other)
		if err != nil {
			t.Fatal(err)
		}

		errs := redeemTogether(discord, member, ownerCode.Code, otherCode.Code)
		if errs[0] != nil || !errors.Is(errs[1], usecase.ErrDiscordAlreadyLinked) {
			t.Fatalf("tour %d : titulaire %v, autre %v ; attendu nil et ErrDiscordAlreadyLinked", round, errs[0], errs[1])
		}
	}
}

// --- Le compte change, le bot l'apprend ------------------------------------

func TestRoleDesactivationEtReactivationParlentAuBot(t *testing.T) {
	discord, events, pool := newDiscord(t)
	accounts, _, _ := newAccounts(t)
	ctx := context.Background()
	admin, _ := createUser(t, pool, "admin")
	userID, _ := createUser(t, pool, "team")
	member := snowflake()
	link(t, discord, userID, member)
	start := headCursor(t, pool)

	if err := accounts.SetRole(ctx, admin, userID, "admin"); err != nil {
		t.Fatalf("changement de role : %v", err)
	}
	if err := accounts.Disable(ctx, admin, userID); err != nil {
		t.Fatalf("desactivation : %v", err)
	}

	// Desactive : la liaison existe pour lui, pas pour le bot.
	if _, err := discord.Link(ctx, member); !errors.Is(err, usecase.ErrDiscordLinkNotFound) {
		t.Fatalf("lecture d'un compte desactive : %v", err)
	}
	page, err := discord.Links(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range page.Items {
		if l.DiscordUserID == member {
			t.Error("un compte desactive parait dans la liste")
		}
	}

	if err := accounts.Enable(ctx, admin, userID); err != nil {
		t.Fatalf("reactivation : %v", err)
	}
	// Reactiver un compte actif ne dit rien.
	if err := accounts.Enable(ctx, admin, userID); err != nil {
		t.Fatal(err)
	}

	wantEvents(t, eventsSince(t, events, start, member),
		seenEvent{Type: usecase.BotEventDiscordLinkUpserted, DiscordUserID: member, Role: "admin"},
		seenEvent{Type: usecase.BotEventDiscordLinkRemoved, DiscordUserID: member},
		seenEvent{Type: usecase.BotEventDiscordLinkUpserted, DiscordUserID: member, Role: "admin"})
}

func TestUnCompteNonLieNEmetRien(t *testing.T) {
	_, events, pool := newDiscord(t)
	accounts, _, _ := newAccounts(t)
	ctx := context.Background()
	admin, _ := createUser(t, pool, "admin")
	userID, _ := createUser(t, pool, "team")
	start := headCursor(t, pool)

	if err := accounts.SetRole(ctx, admin, userID, "admin"); err != nil {
		t.Fatal(err)
	}
	page, err := events.Events(ctx, &start, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("%d evenement(s) pour un compte non lie", len(page.Items))
	}
}

func TestOublierUnCompteEmetDansSaTransactionOuPasDuTout(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "client")
	member := snowflake()
	link(t, discord, userID, member)
	start := headCursor(t, pool)

	// Une suppression annulee n'a rien dit au bot, et la liaison demeure.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := discord.Forget(ctx, tx, userID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if headCursor(t, pool) != start {
		t.Fatal("le compteur a avance malgre l'annulation")
	}
	if _, err := discord.Link(ctx, member); err != nil {
		t.Fatalf("liaison perdue apres annulation : %v", err)
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := discord.Forget(ctx, tx, userID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if export, err := discord.Export(ctx, userID); err != nil || export != nil {
		t.Fatalf("export apres oubli : %+v, %v", export, err)
	}
	wantEvents(t, eventsSince(t, events, start, member),
		seenEvent{Type: usecase.BotEventDiscordLinkRemoved, DiscordUserID: member})
}

func TestLaLiaisonFigureALExportEtPartAvecLeCompte(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "client")
	member := snowflake()
	link(t, discord, userID, member)
	if _, err := discord.CreateCode(ctx, userID); err != nil {
		t.Fatal(err)
	}

	export, err := discord.Export(ctx, userID)
	if err != nil || export == nil || export.DiscordUserID != member {
		t.Fatalf("export = %+v, %v", export, err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	var links, codes int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM discord_links WHERE discord_user_id = $1`, member).Scan(&links)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM discord_link_codes WHERE user_id = $1`, userID).Scan(&codes)
	if links != 0 || codes != 0 {
		t.Fatalf("restent %d liaison(s) et %d code(s) apres la suppression du compte", links, codes)
	}
}

// --- Flux et liste --------------------------------------------------------

func TestLeFluxSeLitParCurseur(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	start := headCursor(t, pool)

	members := []string{snowflake(), snowflake(), snowflake()}
	for _, m := range members {
		userID, _ := createUser(t, pool, "team")
		link(t, discord, userID, m)
	}

	first, err := events.Events(ctx, &start, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("%d evenements, attendu 2", len(first.Items))
	}
	if first.NextAfter != first.Items[1].ID {
		t.Errorf("next_after = %s, attendu %s", first.NextAfter, first.Items[1].ID)
	}
	a, _ := strconv.ParseInt(first.Items[0].ID, 10, 64)
	b, _ := strconv.ParseInt(first.Items[1].ID, 10, 64)
	if a <= start || b <= a {
		t.Errorf("identifiants non croissants : %d puis %d apres %d", a, b, start)
	}

	next, _ := strconv.ParseInt(first.NextAfter, 10, 64)
	second, err := events.Events(ctx, &next, 100)
	if err != nil || len(second.Items) != 1 {
		t.Fatalf("seconde page : %d evenement(s), %v", len(second.Items), err)
	}

	// Page vide : le curseur recu revient tel quel.
	end, _ := strconv.ParseInt(second.NextAfter, 10, 64)
	empty, err := events.Events(ctx, &end, 100)
	if err != nil || len(empty.Items) != 0 || empty.NextAfter != second.NextAfter {
		t.Fatalf("page vide = %+v, %v", empty, err)
	}
}

func TestUnCurseurPurgeEstPerime(t *testing.T) {
	discord, events, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	link(t, discord, userID, snowflake())
	head := headCursor(t, pool)

	// Tout le flux vieillit d'un coup, puis la purge passe.
	if _, err := pool.Exec(ctx, `UPDATE bot_events SET created_at = now() - interval '31 days' WHERE id <= $1`, head); err != nil {
		t.Fatal(err)
	}
	if err := events.Purge(ctx); err != nil {
		t.Fatal(err)
	}

	stale := head - 1
	if _, err := events.Events(ctx, &stale, 10); !errors.Is(err, usecase.ErrEventsCursorExpired) {
		t.Fatalf("curseur purge : %v, attendu ErrEventsCursorExpired", err)
	}
	page, err := events.Events(ctx, &head, 10)
	if err != nil {
		t.Fatalf("curseur a la limite de la purge : %v", err)
	}
	if page.NextAfter != strconv.FormatInt(head, 10) {
		t.Errorf("next_after = %s, attendu %d", page.NextAfter, head)
	}
	fromStart, err := events.Events(ctx, nil, 10)
	if err != nil {
		t.Fatalf("lecture sans curseur apres purge : %v", err)
	}
	if n, _ := strconv.ParseInt(fromStart.NextAfter, 10, 64); n < head {
		t.Errorf("sans curseur, le flux repart de %d, avant la purge (%d)", n, head)
	}
}

func TestLaListeSeParcourtParCurseurEtDonneLeCurseurDuFlux(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()
	for range 3 {
		userID, _ := createUser(t, pool, "client")
		link(t, discord, userID, snowflake())
	}
	head := headCursor(t, pool)

	var all []string
	after := ""
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("la liste ne se termine pas")
		}
		page, err := discord.Links(ctx, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if pages == 0 && page.EventsCursor != strconv.FormatInt(head, 10) {
			t.Errorf("events_cursor = %s, attendu %d", page.EventsCursor, head)
		}
		if len(page.Items) > 2 {
			t.Fatalf("%d liaisons pour une page de 2", len(page.Items))
		}
		for _, l := range page.Items {
			if len(all) > 0 && l.DiscordUserID <= all[len(all)-1] {
				t.Fatalf("ordre rompu : %s apres %s", l.DiscordUserID, all[len(all)-1])
			}
			all = append(all, l.DiscordUserID)
		}
		if page.NextAfter == nil {
			break
		}
		after = *page.NextAfter
	}
	if len(all) < 3 {
		t.Fatalf("%d liaisons parcourues, au moins 3 attendues", len(all))
	}
}

func TestLesCodesExpiresSontPurges(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	if _, err := discord.CreateCode(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE discord_link_codes SET expires_at = now() - interval '1 minute' WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := discord.Purge(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM discord_link_codes WHERE user_id = $1`, userID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d code(s) expire(s) restant(s)", left)
	}
}

func TestLaCarteDitSiLeBotEstBranche(t *testing.T) {
	discord, _, pool := newDiscord(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "client")

	name := "carte-" + uuid.NewString()[:8]
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM bot_keys WHERE name = $1`, name) })
	if _, _, err := usecase.NewBotKeyService(pool).Create(ctx, name); err != nil {
		t.Fatal(err)
	}

	status, err := discord.Status(ctx, userID)
	if err != nil || !status.Available || status.Linked || status.DiscordUserID != nil || status.LinkedAt != nil {
		t.Fatalf("statut = %+v, %v", status, err)
	}
}
