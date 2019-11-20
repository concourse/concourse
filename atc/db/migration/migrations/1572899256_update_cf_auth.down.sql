BEGIN;
  UPDATE teams SET auth = regexp_replace(auth,'"cf:([^"]*):([^"]*):developer"','"cf:\1:\2"','g');
COMMIT;
