# Sankasten

The Sankasten design system: design tokens, the shared control styles, the icon set and the Vue 3 components built on them.

## Use

Import the styles once at startup, then the components from the package entry:

```ts
import 'sankasten/styles/index.css'
import { AppButton, AppInput } from 'sankasten'
```

The fonts the type tokens name (Comfortaa Variable, DM Sans Variable, JetBrains Mono Variable) are bundled by the app, for example from the `@fontsource-variable` packages.

## Develop

```sh
npm install
npm run dev        # the documentation site
npm run typecheck
npm run lint
npm test
npm run build      # the static documentation site into dist/
```

## License

Apache-2.0, see `LICENSE`. Copyright is held by The Sankasten Authors, listed in `AUTHORS`.
