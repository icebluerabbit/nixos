# users

User identity and aspect assembly — the top-level composition point for each user account.

Each aspect defines one user: their username, shell, and the full list of aspects they include. This is the place where everything comes together — a user aspect lists every CLI tool, GUI application, hardware mixin, and service they need, pulling in transitive dependencies automatically via `includes`. There is no intermediate "profile" or "suite" layer; users list their aspects explicitly.
