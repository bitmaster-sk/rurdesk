// Angular TestBed bootstrap for Vitest browser mode.
import '@angular/compiler';
import 'zone.js';
import 'zone.js/testing';
import { afterEach } from 'vitest';
import { getTestBed } from '@angular/core/testing';
import {
    BrowserDynamicTestingModule,
    platformBrowserDynamicTesting
} from '@angular/platform-browser-dynamic/testing';

// Initialize the Angular test environment exactly once. When Vitest runs many
// browser specs in a shared context this setup can be evaluated more than once;
// a second initTestEnvironment call throws "Cannot set base providers because it
// has already been called", which surfaces as an unhandled error and fails
// otherwise-passing files. Guard so it is idempotent.
try {
    getTestBed().initTestEnvironment(BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
    // Test environment already initialized — safe to ignore.
}

// Reset the TestBed after every test so the next spec (especially across files
// sharing one browser context) can call configureTestingModule again. Without
// this, a second file throws "Cannot configure the test module when the test
// module has already been instantiated".
afterEach(() => {
    getTestBed().resetTestingModule();
});
