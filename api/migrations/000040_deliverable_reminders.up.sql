-- Relance d'un livrable qui attend : la date du rappel envoye au client.
--
-- Nulle tant qu'aucune relance n'est partie. Une version n'est relancee
-- qu'une fois : au-dela, c'est un appel, pas un e-mail de plus.
ALTER TABLE deliverable_versions ADD COLUMN reminded_at timestamptz;
