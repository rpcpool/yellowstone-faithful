import { GRPCMethod, RPCMethod } from "./types"

export function buildRPCRequest(
  template: RPCMethod,
  paramValues: Record<string, string>
): string {
  const body = { ...template.example }
  
  if (template.params && template.params.length > 0) {
    // Handle Solana RPC parameter structure
    if (["getBlockTime", "getTransaction", "getSignaturesForAddress", "getBlock"].includes(template.method)) {
      const requiredParams = template.params.filter(p => p.required)
      const optionalParams = template.params.filter(p => !p.required)
      
      body.params = []
      
      // Add required parameters
      requiredParams.forEach(param => {
        const value = paramValues[param.name]
        if (value !== undefined && value !== "") {
          body.params.push(param.type === "number" ? Number(value) : value)
        } else {
          body.params.push(`{{${param.name}}}`)
        }
      })
      
      // Add optional parameters as an object if any are set
      if (optionalParams.length > 0) {
        const optionsObj: Record<string, string | number | boolean> = {}
        let hasOptions = false
        
        optionalParams.forEach(param => {
          const value = paramValues[param.name]
          const finalValue = (value !== undefined && value !== "") ? value : param.default
          
          if (finalValue !== undefined) {
            if (param.type === "number") {
              optionsObj[param.name] = Number(finalValue)
            } else if (param.type === "boolean") {
              optionsObj[param.name] = finalValue === "true" || finalValue === true
            } else {
              optionsObj[param.name] = finalValue as string | number | boolean
            }
            hasOptions = true
          }
        })
        
        if (hasOptions) {
          body.params.push(optionsObj)
        }
      }
    } else {
      // For other methods, use the original logic with defaults
      body.params = template.params.map(param => {
        const value = paramValues[param.name]
        const finalValue = (value !== undefined && value !== "") ? value : param.default
        
        if (finalValue !== undefined) {
          if (param.type === "number") {
            return Number(finalValue)
          } else if (param.type === "boolean") {
            return finalValue === "true" || finalValue === true
          } else {
            return finalValue
          }
        }
        return `{{${param.name}}}`
      })
    }
  }
  
  return JSON.stringify(body, null, 2)
}

export function buildGRPCCommand(
  template: GRPCMethod,
  paramValues: Record<string, string>
): string {
  let command = template.example
  
  // Replace parameter placeholders in the gRPC command
  template.params.forEach(param => {
    const value = paramValues[param.name]
    if (value !== undefined && value !== "") {
      // For gRPC commands, we need to replace the JSON data part
      if (param.type === "number") {
        command = command.replace(
          new RegExp(`"${param.name}":\\s*\\d+`, 'g'),
          `"${param.name}": ${value}`
        )
      } else {
        command = command.replace(
          new RegExp(`"${param.name}":\\s*"[^"]*"`, 'g'),
          `"${param.name}": "${value}"`
        )
      }
    }
  })
  
  return command
} 