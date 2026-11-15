package usecase

import "errors"

// ErrInjected est l'echec que FailAfter provoque.
var ErrInjected = errors.New("echec injecte par le test")

// FailAfter fait echouer l'execution d'un effacement apres l'etape donnee
// (« discord », « user », « exports », « files », « projects », « done ») : un
// test verifie ainsi le retour arriere complet. Compile dans les tests seulement.
func (s *RgpdErasureService) FailAfter(step string) {
	s.afterStep = func(name string) error {
		if name == step {
			return ErrInjected
		}

		return nil
	}
}

// AfterBuild fait executer f apres la construction d'une archive, avant que son
// resultat soit ecrit : un test change ainsi l'etat de l'export en cours de route.
func (s *RgpdExportService) AfterBuild(f func()) { s.afterBuild = f }
