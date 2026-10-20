# Merchant and customer applications

This is the application boundary of the monorepo, separate from backend runtimes
under `services/`. Frontend framework and package tooling are not selected yet.

Merchant setup/catalog administration and order handling, plus the customer
storefront, must use the supported application APIs rather than private database
access or a parallel UI-only purchase lifecycle. Add actual application directories
when building those journeys; do not fill them with untested framework scaffolds.
