-- No-op: validation cannot be undone independently. The constraint itself is
-- dropped/narrowed by the prior migration's down step.
SELECT 1;
