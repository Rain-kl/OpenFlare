import packageJson from "../package.json"

const rawVersion = process.env.NEXT_PUBLIC_APP_VERSION || packageJson.version

export const APP_VERSION = rawVersion.startsWith('v') ? rawVersion.slice(1) : rawVersion
export const APP_BUILD_DATE = process.env.NEXT_PUBLIC_APP_BUILD_DATE || ""
