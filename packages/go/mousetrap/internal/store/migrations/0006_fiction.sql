-- MAM's announce carries a fiction flag ("Fiction" / "Non-Fiction") that autobrr's
-- builtin var vocabulary has nowhere to put — our forked definition routes it through
-- the `origin` capture group to reach the webhook (see definitions/myanonamouse.yaml).
-- It is the only signal separating fiction from non-fiction: MAM reuses category names
-- across the line, with Crime, Historical and Paranormal all observed on both sides.
-- NULL on every pre-existing row and on torznab search results, which carry no flag.
ALTER TABLE announces ADD COLUMN fiction TEXT;
