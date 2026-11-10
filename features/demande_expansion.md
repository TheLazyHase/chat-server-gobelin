But de la demande & contexte
============================

Ajouts pour le bon fonctionnement du projet Papergram.

Papergram est une messagerie où les étudiants d'une classe s'envoient des avions en papier : des feuilles qui voyagent d'une personne à l'autre, avec du texte (et plus tard du dessin) et sur lesquelles on peut répondre. Le front (TypeScript, Vite, React, Three.js) affiche une scène 3D de la classe où apparaissent les camarades connectés.


Les routes existantes (`/login`, `/me`, `/channel/...`) ne sont pas supprimées. Seul l'accès à `POST /user` change (demande 1).

Demande 1 : Proposition technique
=====================

`GET /users` (liste de tous les utilisateurs, proposée par ailleurs côté code) doit être public, c'est-à-dire accessible sans JWT. Il ne renvoie que les champs nécessaires aux cartes, jamais le hash du mot de passe :

```json
[
  {
    "id": 12,
    "username": "Camille",
    "birthday_date": "2003-05-14",
    "photo_url": "/uploads/photos/12.jpg",
    "font_key": "caveat",
    "avatar_key": "avatar-03"
  }
]
```

`POST /user` devrait être fermé, ou réservé à un admin, car la liste de la classe est fixe. Les comptes seraient créés par un script de seed (voir demande 7).

Demande 1 : Utilisation approximative
============================

Je souhaite m'identifier en choisissant une « carte étudiante » dès l'arrivée sur le site, donc avoir les utilisateurs visibles sans devoir m'authentifier. Une fois ma carte choisie, je saisis mon mot de passe et la connexion passe par `POST /login`.

Demande 2 : Proposition technique
=====================

Ajouter à `users` :
- `birthday_date` : DATE, format ISO `YYYY-MM-DD`
- `photo_url` : TEXT, chemin d'une image servie en statique (ex. `/uploads/photos/{id}.jpg`)
- `font_key` : TEXT, clé d'une police choisie dans une liste fournie par le front, assignée à la création du compte et propre à chaque utilisateur
- `avatar_key` : TEXT, nullable, clé du personnage 3D, assignée plus tard

Les colonnes sont nullables pour ne pas casser les comptes existants. Ces champs sont renvoyés par `GET /users` et `GET /me`.

Pas d'endpoint d'upload au début : les photos sont placées par le script de seed.

Demande 2 : Utilisation approximative
============================

Sur les cartes étudiantes, je souhaite afficher une photo de profil et une date d'anniversaire.
Chaque utilisateurice aura par la suite un avatar et une police d'écriture personnalisée, assignés rien qu'à lui ou elle.

Demande 3 : Proposition technique
=====================

Un endpoint WebSocket `GET /ws?token=<jwt>`. Un navigateur ne peut pas envoyer d'en-tête `Authorization` sur un WebSocket, donc le token passe en paramètre d'URL.

La connexion est refusée si le token est invalide, et fermée quand il expire. L'origine de la requête (`Origin`) est vérifiée.

Demande 3 : Utilisation approximative
============================

Garder une connexion ouverte par utilisateur connecté, pour recevoir en direct les événements de présence (demande 4), de papiers (demande 5) et du board (demande 6), sans interroger le back en boucle.

Demande 4 : Proposition technique
=====================

Événements serveur → client :

```json
{ "type": "presence.snapshot", "user_ids": [3, 12] }
{ "type": "presence.join", "user_id": 12 }
{ "type": "presence.leave", "user_id": 12 }
```

`presence.snapshot` est envoyé à la connexion, `join` et `leave` ensuite en direct.

Règles :
- un utilisateur connecté sur deux onglets compte une seule fois ;
- il n'est « parti » qu'à la fermeture de sa dernière connexion ;
- ping/pong régulier pour détecter les connexions mortes.

`GET /presence` renvoie la liste des utilisateurs en ligne.

Demande 4 : Utilisation approximative
============================

La scène 3D n'affiche que les camarades connectés. À l'arrivée d'un nouveau camarade (`join`), une animation d'apparition montre son personnage dans la salle, et il disparaît à son départ (`leave`). Le `snapshot` permet d'afficher directement les personnes déjà présentes à ma connexion. `GET /presence` sert à recharger l'état si ma connexion WebSocket a été coupée.

**Demande 5 (P2) : les avions en papier**

Demande 5 : Proposition technique
=====================

Deux nouvelles tables, à la place de `channels` et `messages` pour cette fonctionnalité :
- `papers` : id, creator_id, holder_id (celui qui l'a en main), status (`unread`, `read`, `discarded`), seed (entier aléatoire), updated_at
- `paper_entries` : id, paper_id, author_id, text, strokes (JSONB, nullable, réservé au dessin), created_at

Endpoints :

| Route | Rôle |
|---|---|
| `POST /papers` `{recipient_id, text, strokes?}` | Crée l'avion et sa première entrée |
| `POST /papers/{id}/reply` `{recipient_id, text, strokes?}` | Ajoute une entrée et envoie à `recipient_id` |
| `GET /papers` | Les papiers que je détiens (ma pile), avec leurs entrées |
| `POST /papers/{id}/read` | Marque comme lu |
| `DELETE /papers/{id}` | Jeté dans la poubelle (suppression logique) |

Événement WebSocket envoyé au destinataire connecté :

```json
{
  "type": "paper.received",
  "paper": {
    "id": 301,
    "status": "unread",
    "seed": 48213,
    "entries": [
      { "id": 1, "author_id": 12, "text": "...", "strokes": null, "created_at": "..." }
    ]
  }
}
```

Format de `strokes` (coordonnées normalisées entre 0 et 1, relatives à la feuille) :

```json
[{ "color": "#000000", "width": 2, "points": [[0.10, 0.20], [0.12, 0.25]] }]
```

Règles :
- seul le détenteur (`holder_id`) peut répondre, marquer comme lu ou jeter un papier ;
- à chaque envoi ou réponse, `holder_id` devient le destinataire, dans une même transaction ;
- `GET /papers` ne renvoie que les papiers détenus et non jetés, avec leurs entrées triées par date ;
- `seed` est fixé à la création et ne change plus (il sert à donner la même forme et la même position à la boule de papier à chaque affichage) ;
- limites proposées, à valider avec les designers : texte de 500 caractères maximum par entrée, 20 entrées maximum par papier, 200 Ko maximum pour `strokes`.

Demande 5 : Utilisation approximative
============================

Un avion est une feuille qui voyage : j'écris dessus, je l'envoie à un.e camarade connecté.e, iel peut répondre sur la même feuille puis la renvoyer à son tour. Les entrées s'accumulent sur la feuille, chacune écrite dans la police de son auteur.
Après lecture, elle rejoint ma pile en bas de l'écran. Je peux ensuite la jeter dans une poubelle (suppression).

`strokes` est prévu dès maintenant, même si le dessin sera ajouté plus tard, pour éviter une migration.

Demande 6 : Proposition technique
=====================

Un channel `board`, accessible à tous les utilisateurs connectés en lecture et en écriture, où l'on peut écrire du texte et dessiner (`strokes`) à des coordonnées précises sur une interface.

Un élément du board contient : `id`, `author_id`, `text` (nullable), `strokes` (nullable, même format que la demande 5), `x` et `y` (coordonnées normalisées entre 0 et 1), `created_at`.

L'implémentation est au choix de l'équipe :
- soit réutiliser `channels` et `messages`, en ajoutant à `messages` les colonnes nullables `x`, `y` et `strokes` (`content` pouvant être vide pour un dessin seul) ;
- soit créer une table dédiée `board_items`.

Dans tous les cas, le front attend :
- `GET /board` : liste des éléments
- `POST /board` `{text?, strokes?, x, y}` : ajoute un élément
- `DELETE /board/{id}` : supprime un élément
- un événement WebSocket à chaque ajout ou suppression :

```json
{ "type": "board.item_created", "item": { "id": 7, "author_id": 12, "text": "...", "strokes": null, "x": 0.4, "y": 0.7 } }
{ "type": "board.item_deleted", "item_id": 7 }
```

Demande 6 : Utilisation approximative
============================

Un grand tableau commun à toute la classe où chacun peut poser un mot ou un dessin à l'endroit voulu, et voir ceux des autres apparaître en direct.

Demande 7 : Proposition technique
=====================

- Script de seed qui crée les comptes de la classe avec `birthday_date`, `photo_url`, `font_key` et `avatar_key`, ainsi que quelques comptes de test.
- Limitation de débit sur `/login`, `POST /papers`, `POST /papers/{id}/reply` et `POST /board`.
- Validation des entrées : longueurs de texte, taille de `strokes`, coordonnées du board entre 0 et 1.
- Documentation de chaque nouvel endpoint dans un `.md`, dans le style de `user.md`, `channel.md` et `message.md`.
- CORS restreint à l'origine du front une fois en production (aujourd'hui `*`).
- Optionnel : `/login` renvoie du JSON `{ "token": "...", "user": {...} }` au lieu du token en texte brut.

Demande 7 : Utilisation approximative
============================

Permettre au front de développer et de tester sans deviner les formats de réponse, et sans que les nouvelles routes puissent être utilisées à mauvais escient.

Problèmes potentiels et/ou effets secondaires
=============================================

**Demande 1**
- La liste publique expose les noms, anniversaires et photos à toute personne qui peut joindre le serveur. Acceptable pour une classe, mais à limiter au minimum de champs (voire jour et mois seulement pour l'anniversaire).
- Fermer `POST /user` désactive le bouton de création de compte du front actuel : le script de seed devient indispensable.

**Demande 2**
- La migration doit rester compatible avec les comptes existants (colonnes nullables).
- `font_key` unique par utilisateur suppose une liste de polices au moins aussi longue que la classe : cette liste doit être fournie par le front avant le seed.
- Les photos servies en statique demandent de limiter le type et la taille des fichiers.

**Demande 3**
- Le token dans l'URL apparaît dans les logs : ne pas journaliser la query string de `/ws`, ou utiliser un ticket à usage unique (un `POST` authentifié qui renvoie un ticket de courte durée, passé ensuite en `?ticket=`).
- Le JWT expire au bout de 3 jours : la connexion WebSocket doit être fermée à l'expiration.
- Un « hub » en mémoire ne fonctionne que sur une seule instance du back. Avec plusieurs instances, il faudrait un système de diffusion partagé (type Redis pub/sub).

**Demande 4**
- Un rafraîchissement de page provoque un `leave` puis un `join` quasi immédiat, donc une disparition et une réapparition du personnage : prévoir un court délai de grâce (2 à 3 secondes) avant d'envoyer `leave`.
- Sans ping/pong, des connexions mortes feraient apparaître des utilisateurs fantômes.

**Demande 5**
- Les papiers destinés à des utilisateurs hors ligne s'accumulent : prévoir une limite ou une pagination sur `GET /papers`.
- Risque de double envoi ou de réponses simultanées : le changement de détenteur doit être atomique.
- `strokes` peut devenir lourd : la limite de taille est nécessaire.
- Les tables `channels` et `messages` restent inchangées et ne sont plus utilisées pour cette fonctionnalité.

**Demande 6**
- Un board accessible à tous en écriture pose des questions de modération (suppression par un admin) et de croissance (limite du nombre d'éléments ou pagination).
- Point à trancher : « publiquement » veut-il dire tous les utilisateurs connectés, ou aussi des visiteurs non connectés en lecture ? Ce document suppose les utilisateurs connectés.
- Si `messages` est réutilisée, `content` est `NOT NULL` : un dessin sans texte doit envoyer une chaîne vide.

**Demande 7**
- Les limites de débit doivent être assez larges pour ne pas bloquer un usage normal (plusieurs réponses rapides dans une conversation).
