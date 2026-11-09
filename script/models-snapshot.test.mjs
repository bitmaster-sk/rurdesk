import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { ModelsSnapshot } from './models-snapshot.mjs';

describe('ModelsSnapshot', () => {
    it('keeps a model thinking when any provider says it reasons', () => {
        const isThinkingByName = ModelsSnapshot.isThinkingByName({
            openai: {
                models: {
                    'gpt-4o': { reasoning: false },
                    'gpt-5': { reasoning: true },
                    'gpt-oss-20b': { reasoning: false }
                }
            },
            openrouter: {
                models: {
                    'openai/GPT-4o': { reasoning: false },
                    'openai/gpt-oss-20b': { reasoning: true },
                    'meta/llama-3.3': {}
                }
            }
        });

        assert.deepEqual(
            [...isThinkingByName],
            [
                ['gpt-4o', false],
                ['gpt-5', true],
                ['gpt-oss-20b', true]
            ]
        );
    });

    it('names a model after its last path segment in lowercase', () => {
        assert.equal(ModelsSnapshot.modelName('openrouter/openai/GPT-4o'), 'gpt-4o');
        assert.equal(ModelsSnapshot.modelName('claude-opus-5-5'), 'claude-opus-5-5');
    });

    it('writes a migration that replaces the snapshot', () => {
        const migration = ModelsSnapshot.migration(
            new Map([
                ['gpt-4o', false],
                ["o'brien", true]
            ])
        );

        assert.equal(
            migration,
            `-- +goose Up
DELETE FROM agent.ai_model;
INSERT INTO agent.ai_model (name, is_thinking) VALUES
    ('gpt-4o', false),
    ('o''brien', true);

-- +goose Down
DELETE FROM agent.ai_model;
`
        );
    });

    it('only clears the table for an empty catalog', () => {
        assert.equal(
            ModelsSnapshot.migration(new Map()),
            `-- +goose Up
DELETE FROM agent.ai_model;

-- +goose Down
DELETE FROM agent.ai_model;
`
        );
    });

    it('stamps the migration with a goose timestamp', () => {
        assert.equal(
            ModelsSnapshot.timestamp(new Date('2026-11-09T05:07:03.120Z')),
            '20261109050703'
        );
    });
});
