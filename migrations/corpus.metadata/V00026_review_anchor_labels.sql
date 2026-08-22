-- ==UP==
ALTER TABLE review_anchors ADD COLUMN label TEXT;

CREATE UNIQUE INDEX idx_review_anchors_work_label
    ON review_anchors(work_id, label)
    WHERE label IS NOT NULL;

-- ==DOWN==
DROP INDEX idx_review_anchors_work_label;
ALTER TABLE review_anchors DROP COLUMN label;
