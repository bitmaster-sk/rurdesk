#!/usr/bin/env node
//
// Writes a migration with a snapshot of the models.dev catalog into agent.ai_model.
// The app never calls models.dev itself; review the migration like any other.
//
// Usage:  node script/models-snapshot.mjs
//         node script/models-snapshot.mjs --source api.json    # from a downloaded catalog
import { readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { parseArgs } from 'node:util';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');

export class ModelsSnapshot {
    // Must match service.ModelName in the API, which normalises the gateway model before the lookup.
    static modelName(id) {
        return id.slice(id.lastIndexOf('/') + 1).toLowerCase();
    }

    static isThinkingByName(catalog) {
        const isThinkingByName = new Map();
        for (const provider of Object.values(catalog)) {
            for (const [id, model] of Object.entries(provider.models ?? {})) {
                if (typeof model.reasoning !== 'boolean') {
                    continue;
                }
                const name = ModelsSnapshot.modelName(id);
                isThinkingByName.set(name, isThinkingByName.get(name) === true || model.reasoning);
            }
        }
        return new Map([...isThinkingByName].sort(([left], [right]) => (left < right ? -1 : 1)));
    }

    static migration(isThinkingByName) {
        const rows = [...isThinkingByName].map(
            ([name, isThinking]) => `    ('${name.replaceAll("'", "''")}', ${isThinking})`
        );
        const insert = rows.length
            ? `INSERT INTO agent.ai_model (name, is_thinking) VALUES\n${rows.join(',\n')};\n`
            : '';
        return `-- +goose Up\nDELETE FROM agent.ai_model;\n${insert}\n-- +goose Down\nDELETE FROM agent.ai_model;\n`;
    }

    static async readSource(source) {
        if (!/^https?:\/\//.test(source)) {
            return JSON.parse(await readFile(source, 'utf8'));
        }
        const response = await fetch(source);
        if (!response.ok) {
            throw new Error(`fetching ${source}: HTTP ${response.status}`);
        }
        return response.json();
    }

    static timestamp(date) {
        return date.toISOString().replace(/\D/g, '').slice(0, 14);
    }

    static async main() {
        const { values } = parseArgs({
            options: {
                source: { type: 'string', default: 'https://models.dev/api.json' },
                dir: { type: 'string', default: join(ROOT, 'api', 'migrations') }
            }
        });
        const isThinkingByName = ModelsSnapshot.isThinkingByName(
            await ModelsSnapshot.readSource(values.source)
        );
        const path = join(
            values.dir,
            `${ModelsSnapshot.timestamp(new Date())}_ai_model_snapshot.sql`
        );
        await writeFile(path, ModelsSnapshot.migration(isThinkingByName));
        console.log(`wrote ${isThinkingByName.size} models to ${path}`);
    }
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
    ModelsSnapshot.main().catch(error => {
        console.error(`error: ${error.message}`);
        process.exit(1);
    });
}
