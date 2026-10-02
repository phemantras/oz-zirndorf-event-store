-- +goose Up
-- Baseline migration: it only establishes goose version tracking and holds
-- no structure, because domain tables belong to the stories that introduce
-- them (in later, forward-only migrations).
-- Once applied, this file must never change (AD-17).
SELECT 1;
