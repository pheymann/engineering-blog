import { cp, mkdir, realpath, rm } from 'node:fs/promises';
import { basename, dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sourceDirectory = resolve(projectRoot, 'src/site');
const outputDirectory = resolve(process.env.SITE_DIRECTORY ?? join(projectRoot, 'dist'));

async function canonicalizePath(path) {
  let existingPath = path;
  const missingSegments = [];

  while (true) {
    try {
      return resolve(await realpath(existingPath), ...missingSegments.reverse());
    } catch (error) {
      if (error.code !== 'ENOENT') {
        throw error;
      }

      const parentPath = dirname(existingPath);
      if (parentPath === existingPath) {
        throw error;
      }

      missingSegments.push(basename(existingPath));
      existingPath = parentPath;
    }
  }
}

function containsPath(parentPath, childPath) {
  const pathFromParent = relative(parentPath, childPath);
  return pathFromParent === '' ||
    (!pathFromParent.startsWith(`..${sep}`) && pathFromParent !== '..' && !isAbsolute(pathFromParent));
}

const canonicalSourceDirectory = await canonicalizePath(sourceDirectory);
const canonicalOutputDirectory = await canonicalizePath(outputDirectory);

if (containsPath(canonicalOutputDirectory, canonicalSourceDirectory) ||
    containsPath(canonicalSourceDirectory, canonicalOutputDirectory)) {
  throw new Error(`Refusing to build into ${outputDirectory}: output must not overlap the source directory.`);
}

await rm(outputDirectory, { recursive: true, force: true });
await mkdir(outputDirectory, { recursive: true });
await cp(sourceDirectory, outputDirectory, { recursive: true });

console.log(`Built static site in ${outputDirectory}`);
