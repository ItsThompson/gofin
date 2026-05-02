
// Windows temporarily needs this file, https://github.com/module-federation/vite/issues/68

    import {loadShare} from "@module-federation/runtime";
    const importMap = {
      
        "react": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__react__prebuild__.js");
            return pkg;
        }
      ,
        "react-dom": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__react_mf_2_dom__prebuild__.js");
            return pkg;
        }
      ,
        "react-dom/client": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__react_mf_2_dom_mf_1_client__prebuild__.js");
            return pkg;
        }
      ,
        "react-router": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__react_mf_2_router__prebuild__.js");
            return pkg;
        }
      ,
        "react/jsx-runtime": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__react_mf_1_jsx_mf_2_runtime__prebuild__.js");
            return pkg;
        }
      ,
        "zustand": async () => {
          let pkg = await import("__mf__virtual/__mfe_internal__finance__prebuild__zustand__prebuild__.js");
            return pkg;
        }
      
    }
      const usedShared = {
      
          "react": {
            name: "react",
            version: "19.2.5",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"react"}' must be provided by host`);
              }
              usedShared["react"].loaded = true
              const {"react": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "react" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^19",
              
            }
          }
        ,
          "react-dom": {
            name: "react-dom",
            version: "19.2.5",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"react-dom"}' must be provided by host`);
              }
              usedShared["react-dom"].loaded = true
              const {"react-dom": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "react-dom" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^19",
              
            }
          }
        ,
          "react-dom/client": {
            name: "react-dom/client",
            version: "19.2.5",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"react-dom/client"}' must be provided by host`);
              }
              usedShared["react-dom/client"].loaded = true
              const {"react-dom/client": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "react-dom/client" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^19",
              
            }
          }
        ,
          "react-router": {
            name: "react-router",
            version: "7.14.2",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"react-router"}' must be provided by host`);
              }
              usedShared["react-router"].loaded = true
              const {"react-router": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "react-router" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^7",
              
            }
          }
        ,
          "react/jsx-runtime": {
            name: "react/jsx-runtime",
            version: "19.2.5",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"react/jsx-runtime"}' must be provided by host`);
              }
              usedShared["react/jsx-runtime"].loaded = true
              const {"react/jsx-runtime": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "react/jsx-runtime" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^19",
              
            }
          }
        ,
          "zustand": {
            name: "zustand",
            version: "5.0.12",
            scope: ["default"],
            loaded: false,
            from: "__mfe_internal__finance",
            async get () {
              if (false) {
                throw new Error(`[Module Federation] Shared module '${"zustand"}' must be provided by host`);
              }
              usedShared["zustand"].loaded = true
              const {"zustand": pkgDynamicImport} = importMap
              const res = await pkgDynamicImport()
              const exportModule = false && "zustand" === "react"
                ? (res?.default ?? res)
                : {...res}
              // All npm packages pre-built by vite will be converted to esm
              Object.defineProperty(exportModule, "__esModule", {
                value: true,
                enumerable: false
              })
              return function () {
                return exportModule
              }
            },
            shareConfig: {
              singleton: true,
              requiredVersion: "^5",
              
            }
          }
        
    }
      const usedRemotes = [
      ]
      export {
        usedShared,
        usedRemotes
      }
      