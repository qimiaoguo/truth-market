ALTER TABLE markets ADD COLUMN IF NOT EXISTS resolved_outcome_id UUID REFERENCES outcomes(id);
