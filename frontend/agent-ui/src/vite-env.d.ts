/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_AGENT_FRAME_API_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
