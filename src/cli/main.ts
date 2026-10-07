#!/usr/bin/env node
import module from 'node:module';

// Cache compiled bytecode between runs (Node >= 22.1): cuts CLI startup noticeably.
module.enableCompileCache?.();

const { run } = await import('./index.js');
await run();
