UPDATE deployments SET outcome = 'failure' WHERE outcome = 'in_progress';
ALTER TABLE deployments DROP CONSTRAINT deployments_outcome_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_outcome_check CHECK (outcome IN ('success', 'failure'));
