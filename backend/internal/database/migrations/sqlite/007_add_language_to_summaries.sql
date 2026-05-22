-- Add language column to store user-selected language preference for summary generation
ALTER TABLE summaries ADD COLUMN language VARCHAR(20) DEFAULT '';
