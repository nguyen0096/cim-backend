-- Enable unaccent extension for Vietnamese fuzzy search
-- This extension removes accents from text, allowing searches like "pho" to match "Phở"
-- Required for fuzzy search functionality in menu items and other Vietnamese text fields
-- Reference: https://www.postgresql.org/docs/current/unaccent.html
CREATE EXTENSION IF NOT EXISTS unaccent;

