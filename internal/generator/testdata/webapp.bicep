// Example: Hub & Spoke IaaS Architecture
// - Hub VNet: Azure Firewall, VPN Gateway, Azure Bastion
// - Spoke 1 VNet: Web tier (2× VM behind public Load Balancer)
// - Spoke 2 VNet: App tier (VM) + Data tier (SQL Server/DB)
// - Spoke 3 VNet: AI tier (Azure OpenAI via Private Endpoint)
// - Shared: Key Vault, Diagnostics Storage, Log Analytics
//
// Run: azdiagram examples/webapp.bicep

param location string = 'japaneast'
param prefix string = 'hs'

// ════════════════════════════════════════════════════════════════════
// Hub VNet
// ════════════════════════════════════════════════════════════════════

resource hubVnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: '${prefix}-hub-vnet'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.0.0.0/16']
    }
    subnets: [
      {
        name: 'GatewaySubnet'
        properties: {
          addressPrefix: '10.0.0.0/27'
        }
      }
      {
        name: 'AzureFirewallSubnet'
        properties: {
          addressPrefix: '10.0.1.0/26'
        }
      }
      {
        name: 'AzureBastionSubnet'
        properties: {
          addressPrefix: '10.0.2.0/27'
        }
      }
      {
        name: 'mgmt-subnet'
        properties: {
          addressPrefix: '10.0.3.0/24'
          routeTable: {
            id: hubRouteTable.id
          }
        }
      }
    ]
  }
}

// ── Hub: Azure Firewall ───────────────────────────────────────────

resource hubFwPip 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: '${prefix}-fw-pip'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicIPAllocationMethod: 'Static'
  }
}

resource hubFw 'Microsoft.Network/azureFirewalls@2023-11-01' = {
  name: '${prefix}-fw'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'fw-ipconfig'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/AzureFirewallSubnet'
          }
          publicIPAddress: {
            id: hubFwPip.id
          }
        }
      }
    ]
  }
}

// ── Hub: VPN Gateway ─────────────────────────────────────────────

resource hubGwPip 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: '${prefix}-gw-pip'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicIPAllocationMethod: 'Static'
  }
}

resource hubVpnGw 'Microsoft.Network/virtualNetworkGateways@2023-11-01' = {
  name: '${prefix}-vpngw'
  location: location
  properties: {
    gatewayType: 'Vpn'
    vpnType: 'RouteBased'
    sku: {
      name: 'VpnGw2'
      tier: 'VpnGw2'
    }
    ipConfigurations: [
      {
        name: 'gw-ipconfig'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/GatewaySubnet'
          }
          publicIPAddress: {
            id: hubGwPip.id
          }
        }
      }
    ]
  }
}

// ── Hub: Azure Bastion ───────────────────────────────────────────

resource hubBastionPip 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: '${prefix}-bastion-pip'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicIPAllocationMethod: 'Static'
  }
}

resource hubBastion 'Microsoft.Network/bastionHosts@2023-11-01' = {
  name: '${prefix}-bastion'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    ipConfigurations: [
      {
        name: 'bastion-ipconfig'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/AzureBastionSubnet'
          }
          publicIPAddress: {
            id: hubBastionPip.id
          }
        }
      }
    ]
  }
}

// ── Hub: Route Table (UDR → Firewall) ────────────────────────────

resource hubRouteTable 'Microsoft.Network/routeTables@2023-11-01' = {
  name: '${prefix}-hub-rt'
  location: location
  properties: {
    routes: [
      {
        name: 'default-to-fw'
        properties: {
          addressPrefix: '0.0.0.0/0'
          nextHopType: 'VirtualAppliance'
          nextHopIpAddress: '10.0.1.4'
        }
      }
    ]
  }
}

// ── Hub: Management VMs (mgmt-subnet) ───────────────────────────

resource hubMgmtNic1 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-mgmt-nic1'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/mgmt-subnet'
          }
        }
      }
    ]
  }
}

resource hubMgmtNic2 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-mgmt-nic2'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/mgmt-subnet'
          }
        }
      }
    ]
  }
}

resource hubMgmtNic3 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-mgmt-nic3'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/mgmt-subnet'
          }
        }
      }
    ]
  }
}

resource hubMgmtVm1 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-mgmt-vm1'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: hubMgmtNic1.id
        }
      ]
    }
  }
}

resource hubMgmtVm2 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-mgmt-vm2'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: hubMgmtNic2.id
        }
      ]
    }
  }
}

resource hubMgmtVm3 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-mgmt-vm3'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: hubMgmtNic3.id
        }
      ]
    }
  }
}

// ════════════════════════════════════════════════════════════════════
// Spoke 1: Web Tier
// ════════════════════════════════════════════════════════════════════

resource spoke1Vnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: '${prefix}-spoke1-vnet'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.1.0.0/16']
    }
    subnets: [
      {
        name: 'web-subnet'
        properties: {
          addressPrefix: '10.1.1.0/24'
          networkSecurityGroup: {
            id: spoke1Nsg.id
          }
          routeTable: {
            id: hubRouteTable.id
          }
        }
      }
      {
        name: 'api-subnet'
        properties: {
          addressPrefix: '10.1.2.0/24'
          networkSecurityGroup: {
            id: spoke1Nsg.id
          }
          routeTable: {
            id: hubRouteTable.id
          }
        }
      }
    ]
  }
}

resource spoke1Nsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: '${prefix}-spoke1-nsg'
  location: location
  properties: {
    securityRules: [
      {
        name: 'allow-http'
        properties: {
          priority: 100
          protocol: 'Tcp'
          access: 'Allow'
          direction: 'Inbound'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: '*'
          destinationPortRange: '80'
        }
      }
      {
        name: 'allow-https'
        properties: {
          priority: 110
          protocol: 'Tcp'
          access: 'Allow'
          direction: 'Inbound'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: '*'
          destinationPortRange: '443'
        }
      }
    ]
  }
}

resource webLb 'Microsoft.Network/loadBalancers@2023-11-01' = {
  name: '${prefix}-web-lb'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    frontendIPConfigurations: [
      {
        name: 'web-frontend'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/web-subnet'
          }
        }
      }
    ]
    backendAddressPools: [
      {
        name: 'web-backend'
      }
    ]
  }
}

resource webNic1 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-web-nic1'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/web-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', webLb.name, 'web-backend')
            }
          ]
        }
      }
    ]
  }
}

resource webVm2Nsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: '${prefix}-web-vm2-nsg'
  location: location
  properties: {
    securityRules: []
  }
}

resource webNic2 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-web-nic2'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/web-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', webLb.name, 'web-backend')
            }
          ]
        }
      }
    ]
  }
}

resource webVm1 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-web-vm1'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: webNic1.id
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: true
      }
    }
  }
}

resource webVm2 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-web-vm2'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: webNic2.id
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: true
      }
    }
  }
}

resource webNic3 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-web-nic3'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/web-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', webLb.name, 'web-backend')
            }
          ]
        }
      }
    ]
    networkSecurityGroup: {
      id: spoke1Nsg.id
    }
  }
}

resource webVm3 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-web-vm3'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: webNic3.id
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: true
      }
    }
  }
}

resource spoke2Vnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: '${prefix}-spoke2-vnet'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.2.0.0/16']
    }
    subnets: [
      {
        name: 'app-subnet'
        properties: {
          addressPrefix: '10.2.1.0/24'
          networkSecurityGroup: {
            id: spoke2Nsg.id
          }
          routeTable: {
            id: hubRouteTable.id
          }
        }
      }
      {
        name: 'data-subnet'
        properties: {
          addressPrefix: '10.2.2.0/24'
        }
      }
    ]
  }
}

resource spoke2Nsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: '${prefix}-spoke2-nsg'
  location: location
}

resource appNic1 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-app-nic1'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke2Vnet.id}/subnets/app-subnet'
          }
        }
      }
    ]
    networkSecurityGroup: {
      id: spoke2Nsg.id
    }
  }
}

resource appNic2 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-app-nic2'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke2Vnet.id}/subnets/app-subnet'
          }
        }
      }
    ]
  }
}

resource appVm1 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-app-vm1'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D4s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: appNic1.id
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: true
      }
    }
  }
}

resource appVm2 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-app-vm2'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D4s_v3'
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: appNic2.id
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: true
      }
    }
  }
}

// ── Internal Load Balancer (Spoke1 api-subnet) ───────────────────

resource apiIlb 'Microsoft.Network/loadBalancers@2023-11-01' = {
  name: '${prefix}-api-ilb'
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    frontendIPConfigurations: [
      {
        name: 'api-frontend'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/api-subnet'
          }
          privateIPAllocationMethod: 'Dynamic'
        }
      }
    ]
    backendAddressPools: [
      {
        name: 'api-backend'
      }
    ]
  }
}

resource apiNic1 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-api-nic1'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/api-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', apiIlb.name, 'api-backend')
            }
          ]
        }
      }
    ]
    networkSecurityGroup: {
      id: spoke1Nsg.id
    }
  }
}

resource apiNic2 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-api-nic2'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/api-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', apiIlb.name, 'api-backend')
            }
          ]
        }
      }
    ]
    networkSecurityGroup: {
      id: spoke1Nsg.id
    }
  }
}

resource apiNic3 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: '${prefix}-api-nic3'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: '${spoke1Vnet.id}/subnets/api-subnet'
          }
          loadBalancerBackendAddressPools: [
            {
              id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', apiIlb.name, 'api-backend')
            }
          ]
        }
      }
    ]
  }
}

resource apiVm1 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-api-vm1'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: apiNic1.id
        }
      ]
    }
  }
}

resource apiVm2 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-api-vm2'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: apiNic2.id
        }
      ]
    }
  }
}

resource apiVm3 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: '${prefix}-api-vm3'
  location: location
  properties: {
    hardwareProfile: {
      vmSize: 'Standard_D2s_v3'
    }
    storageProfile: {
      osDisk: {
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Premium_LRS'
        }
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: apiNic3.id
        }
      ]
    }
  }
}

// ── Data: SQL Server + Database ──────────────────────────────────

resource sqlServer 'Microsoft.Sql/servers@2023-08-01-preview' = {
  name: '${prefix}-sql'
  location: location
  properties: {
    administratorLogin: 'sqladmin'
  }
}

resource sqlDb 'Microsoft.Sql/servers/databases@2023-08-01-preview' = {
  parent: sqlServer
  name: 'appdb'
  location: location
  sku: {
    name: 'GeneralPurpose'
    tier: 'GeneralPurpose'
    capacity: 4
  }
}

// ── Private Endpoint for SQL (in data-subnet) ────────────────────

resource sqlPe 'Microsoft.Network/privateEndpoints@2023-11-01' = {
  name: '${prefix}-sql-pe'
  location: location
  properties: {
    subnet: {
      id: '${spoke2Vnet.id}/subnets/data-subnet'
    }
    privateLinkServiceConnections: [
      {
        name: 'sql-plsc'
        properties: {
          privateLinkServiceId: sqlServer.id
          groupIds: ['sqlServer']
        }
      }
    ]
  }
}

// ── Private Endpoint for Key Vault (in data-subnet) ──────────────

resource kvPe 'Microsoft.Network/privateEndpoints@2023-11-01' = {
  name: '${prefix}-kv-pe'
  location: location
  properties: {
    subnet: {
      id: '${spoke2Vnet.id}/subnets/data-subnet'
    }
    privateLinkServiceConnections: [
      {
        name: 'kv-plsc'
        properties: {
          privateLinkServiceId: keyVault.id
          groupIds: ['vault']
        }
      }
    ]
  }
}

// ════════════════════════════════════════════════════════════════════
// Spoke 3: AI Tier
// ════════════════════════════════════════════════════════════════════

resource spoke3Vnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: '${prefix}-spoke3-vnet'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.3.0.0/16']
    }
    subnets: [
      {
        name: 'app-vnetint-subnet'
        properties: {
          addressPrefix: '10.3.3.0/24'
          delegations: [
            {
              name: 'delegation'
              properties: {
                serviceName: 'Microsoft.Web/serverFarms'
              }
            }
          ]
        }
      }
      {
        name: 'aoai-subnet'
        properties: {
          addressPrefix: '10.3.1.0/24'
          routeTable: {
            id: hubRouteTable.id
          }
        }
      }
      {
        name: 'app-pe-subnet'
        properties: {
          addressPrefix: '10.3.2.0/24'
        }
      }
    ]
  }
}

// ── Azure OpenAI Service ─────────────────────────────────────────

resource aoai 'Microsoft.CognitiveServices/accounts@2023-05-01' = {
  name: '${prefix}-aoai'
  location: location
  kind: 'OpenAI'
  sku: {
    name: 'S0'
  }
  properties: {
    publicNetworkAccess: 'Disabled'
    customSubDomainName: '${prefix}-aoai'
  }
}

// ── Private Endpoint for Azure OpenAI (in aoai-subnet) ──────────

resource aoaiPe 'Microsoft.Network/privateEndpoints@2023-11-01' = {
  name: '${prefix}-aoai-pe'
  location: location
  properties: {
    subnet: {
      id: '${spoke3Vnet.id}/subnets/aoai-subnet'
    }
    privateLinkServiceConnections: [
      {
        name: 'aoai-plsc'
        properties: {
          privateLinkServiceId: aoai.id
          groupIds: ['account']
        }
      }
    ]
  }
}

// ── App Service (VNet integration into app-vnetint-subnet) ───────

resource spoke3AppPlan 'Microsoft.Web/serverfarms@2023-01-01' = {
  name: '${prefix}-spoke3-plan'
  location: location
  sku: {
    name: 'P1v3'
    tier: 'PremiumV3'
  }
}

resource spoke3App 'Microsoft.Web/sites@2023-01-01' = {
  name: '${prefix}-spoke3-app'
  location: location
  kind: 'app'
  properties: {
    serverFarmId: spoke3AppPlan.id
    virtualNetworkSubnetId: '${spoke3Vnet.id}/subnets/app-vnetint-subnet'
    vnetRouteAllEnabled: true
    publicNetworkAccess: 'Disabled'
  }
}

// ── Private Endpoint for App Service (in app-pe-subnet) ──────────

resource spoke3AppPe 'Microsoft.Network/privateEndpoints@2023-11-01' = {
  name: '${prefix}-app-pe'
  location: location
  properties: {
    subnet: {
      id: '${spoke3Vnet.id}/subnets/app-pe-subnet'
    }
    privateLinkServiceConnections: [
      {
        name: 'app-plsc'
        properties: {
          privateLinkServiceId: spoke3App.id
          groupIds: ['sites']
        }
      }
    ]
  }
}

// ════════════════════════════════════════════════════════════════════
// VNet Peerings
// ════════════════════════════════════════════════════════════════════
resource hubToSpoke1Peering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: hubVnet
  name: 'hub-to-spoke1'
  properties: {
    remoteVirtualNetwork: {
      id: spoke1Vnet.id
    }
    allowForwardedTraffic: true
    allowGatewayTransit: true
  }
}

resource spoke1ToHubPeering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: spoke1Vnet
  name: 'spoke1-to-hub'
  properties: {
    remoteVirtualNetwork: {
      id: hubVnet.id
    }
    allowForwardedTraffic: true
    useRemoteGateways: true
  }
}

resource hubToSpoke2Peering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: hubVnet
  name: 'hub-to-spoke2'
  properties: {
    remoteVirtualNetwork: {
      id: spoke2Vnet.id
    }
    allowForwardedTraffic: true
    allowGatewayTransit: true
  }
}

resource spoke2ToHubPeering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: spoke2Vnet
  name: 'spoke2-to-hub'
  properties: {
    remoteVirtualNetwork: {
      id: hubVnet.id
    }
    allowForwardedTraffic: true
    useRemoteGateways: true
  }
}

resource hubToSpoke3Peering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: hubVnet
  name: 'hub-to-spoke3'
  properties: {
    remoteVirtualNetwork: {
      id: spoke3Vnet.id
    }
    allowForwardedTraffic: true
    allowGatewayTransit: true
  }
}

resource spoke3ToHubPeering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: spoke3Vnet
  name: 'spoke3-to-hub'
  properties: {
    remoteVirtualNetwork: {
      id: hubVnet.id
    }
    allowForwardedTraffic: true
    useRemoteGateways: true
  }
}

// ════════════════════════════════════════════════════════════════════
// Shared Services
// ════════════════════════════════════════════════════════════════════

resource diagStorage 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: '${prefix}diagsa'
  location: location
  kind: 'StorageV2'
  sku: {
    name: 'Standard_LRS'
  }
}

resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' = {
  name: '${prefix}-kv'
  location: location
  properties: {
    tenantId: subscription().tenantId
    sku: {
      family: 'A'
      name: 'standard'
    }
    enableSoftDelete: true
    softDeleteRetentionInDays: 90
  }
}

resource managedIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: '${prefix}-mi'
  location: location
}

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2023-09-01' = {
  name: '${prefix}-law'
  location: location
  properties: {
    retentionInDays: 90
    sku: {
      name: 'PerGB2018'
    }
  }
}

resource appInsights 'Microsoft.Insights/components@2020-02-02' = {
  name: '${prefix}-ai'
  location: location
  kind: 'web'
  properties: {
    Application_Type: 'web'
    WorkspaceResourceId: logAnalytics.id
  }
}
