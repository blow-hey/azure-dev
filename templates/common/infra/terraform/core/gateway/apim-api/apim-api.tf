terraform {
  required_providers {
    azurerm   = {
      version           = "~>3.18.0"
      source            = "hashicorp/azurerm"
    }
    azurecaf  = {
      source            = "aztfmod/azurecaf"
      version           = "~>1.2.15"
    }
  }
}

data "azurerm_api_management" "myapim"{
  name                  = var.name
  resource_group_name   = var.rg_name
}

resource "azurerm_api_management_logger" "logger"{
  name                  = "app-insights-logger"
  resource_group_name   = var.rg_name
  api_management_name   = data.azurerm_api_management.myapim.name
}

# ------------------------------------------------------------------------------------------------------
# Deploy apim-api service 
# ------------------------------------------------------------------------------------------------------
resource "azurerm_api_management_api" "api" {
  name                  = var.apiName
  resource_group_name   = var.rg_name
  api_management_name   = data.azurerm_api_management.myapim.name
  revision              = "1"
  display_name          = var.apiDisplayName
  path                  = var.apiPath
  protocols             = [ "https"]

  import {
    content_format      = "openapi"
    content_value       = "../../../../api/common/openapi.yaml"
  }
}

resource "azurerm_api_management_api_policy" "policies"{
  api_name              = azurerm_api_management_api.api.name
  api_management_name   = azurerm_api_management_api.api.api_management_name
  resource_group_name   = var.rg_name

  xml_content           = "../../../../../../common/infra/terraform/core/gateway/apim/apim-api-policy.xml"
}

resource "azurerm_api_management_api_diagnostic" "diagnostics"{
  identifier                = "applicationinsights"
  resource_group_name       = var.rg_name
  api_management_name       = azurerm_api_management_api.api.api_management_name
  api_name                  = azurerm_api_management_api.api.name
  api_management_logger_id  = azurerm_api_management_logger.logger.id

  sampling_percentage       = 100.0
  always_log_errors = true
  log_client_ip  = true
  verbosity                 = "verbose"
  http_correlation_protocol = "W3C"

  frontend_request {
    body_bytes = 1024
    headers_to_log = [
      "content-type",
      "accept",
      "origin",
    ]
  }

  frontend_response {
    body_bytes      = 1024
    headers_to_log  = [
      "content-type",
      "content-length",
      "origin",
    ]
  }

  backend_request {
    body_bytes      = 32
    headers_to_log  = [
      "content-type",
      "accept",
      "origin",
    ]
  }

  backend_response {
    body_bytes      = 32
    headers_to_log  = [
      "content-type",
      "content-length",
      "origin",
    ]
  }
}
