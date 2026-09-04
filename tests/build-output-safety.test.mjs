import assert from 'node:assert/strict';
import { access, cp, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

test('build refuses an output directory that contains the source tree', async (context) => {
  const fixtureRoot = await mkdtemp(join(tmpdir(), 'engineering-blog-build-safety-'));
  context.after(() => rm(fixtureRoot, { recursive: true, force: true }));

  await mkdir(join(fixtureRoot, 'scripts'), { recursive: true });
  await mkdir(join(fixtureRoot, 'src/site'), { recursive: true });
  await cp(join(projectRoot, 'scripts/build.mjs'), join(fixtureRoot, 'scripts/build.mjs'));
  const sourceMarker = join(fixtureRoot, 'src/site/index.html');
  await writeFile(sourceMarker, '<!doctype html>');

  const result = spawnSync(process.execPath, ['scripts/build.mjs'], {
    cwd: fixtureRoot,
    env: { ...process.env, SITE_DIRECTORY: fixtureRoot },
    encoding: 'utf8'
  });

  assert.notEqual(result.status, 0, 'an unsafe output directory must be rejected');
  await access(sourceMarker);
});
