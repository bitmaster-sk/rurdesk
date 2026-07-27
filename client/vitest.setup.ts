// Enables Angular's JIT compiler so DI-coupled @Injectable services (and partially
// compiled libraries like @ngx-translate/core) can be resolved without AOT.
// Needed because bare Vitest uses esbuild, which does not run the Angular compiler.
import '@angular/compiler';
