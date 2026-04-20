ALTER TABLE task_caches ADD COLUMN last_used timestamp with time zone NOT NULL DEFAULT now();
ALTER TABLE task_caches ADD COLUMN ttl bigint NOT NULL DEFAULT 0;
