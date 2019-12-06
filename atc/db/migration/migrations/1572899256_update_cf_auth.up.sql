BEGIN;
  UPDATE teams SET auth = regexp_replace(auth, '"cf:([^"]*):([^"]*)"', '"cf:\1:\2:developer"', 'g');
COMMIT;
