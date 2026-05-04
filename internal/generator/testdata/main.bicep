@description('4文字のランダムサフィックス。デフォルトはデプロイ時に自動生成されます。')
param suffix string = substring(uniqueString(resourceGroup().id), 0, 4)

@description('リソースの場所')
param location string = 'japaneast'

@description('VM の管理者ユーザー名')
param adminUsername string

@description('VM の管理者パスワード (12文字以上、大文字・小文字・数字・記号を含む)')
@secure()
@minLength(12)
param adminPassword string


// =============================================================
//  Hub VNet
// =============================================================
resource hubVnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: 'vnet-hub-jpe-${suffix}'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.0.0.0/16']
    }
    subnets: [
      {
        name: 'AzureFirewallSubnet'
        properties: { addressPrefix: '10.0.1.0/26' }
      }
      {
        name: 'AzureFirewallManagementSubnet'
        properties: { addressPrefix: '10.0.1.64/26' }
      }
      {
        name: 'snet-mgmt-jpe'
        properties: { addressPrefix: '10.0.2.0/24' }
      }
    ]
  }
}

// =============================================================
//  Azure Firewall — パブリック IP
// =============================================================
resource afwPip 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: 'pip-afw-jpe-${suffix}'
  location: location
  sku: { name: 'Standard' }
  properties: { publicIPAllocationMethod: 'Static' }
}

resource afwMgmtPip 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: 'pip-afw-mgmt-jpe-${suffix}'
  location: location
  sku: { name: 'Standard' }
  properties: { publicIPAllocationMethod: 'Static' }
}

// =============================================================
//  Azure Firewall Policy (Basic) + アプリケーションルール
// =============================================================
resource afwPolicy 'Microsoft.Network/firewallPolicies@2023-11-01' = {
  name: 'afwp-hub-jpe-${suffix}'
  location: location
  properties: {
    sku: { tier: 'Basic' }
  }
}

resource afwRuleCollectionGroup 'Microsoft.Network/firewallPolicies/ruleCollectionGroups@2023-11-01' = {
  parent: afwPolicy
  name: 'DefaultApplicationRuleCollectionGroup'
  properties: {
    priority: 200
    ruleCollections: [
      {
        ruleCollectionType: 'FirewallPolicyFilterRuleCollection'
        name: 'rc-app-allow'
        priority: 100
        action: { type: 'Allow' }
        rules: [
          {
            ruleType: 'ApplicationRule'
            name: 'allow-ubuntu-apt'
            sourceAddresses: ['10.1.0.0/16']
            targetFqdns: [
              'archive.ubuntu.com'
              'security.ubuntu.com'
              'azure.archive.ubuntu.com'
              'packages.microsoft.com'
              'ppa.launchpad.net'
            ]
            protocols: [
              { protocolType: 'Http',  port: 80  }
              { protocolType: 'Https', port: 443 }
            ]
          }
          {
            ruleType: 'ApplicationRule'
            name: 'allow-ifconfig-me'
            sourceAddresses: ['10.1.0.0/16']
            targetFqdns: ['ifconfig.me']
            protocols: [
              { protocolType: 'Http',  port: 80  }
              { protocolType: 'Https', port: 443 }
            ]
          }
        ]
      }
    ]
  }
}

// =============================================================
//  Azure Firewall Basic
// =============================================================
resource azFirewall 'Microsoft.Network/azureFirewalls@2023-11-01' = {
  name: 'afw-hub-jpe-${suffix}'
  location: location
  properties: {
    sku: {
      name: 'AZFW_VNet'
      tier: 'Basic'
    }
    firewallPolicy: {
      id: afwPolicy.id
    }
    ipConfigurations: [
      {
        name: 'ipconfig-afw'
        properties: {
          subnet: {
            id: '${hubVnet.id}/subnets/AzureFirewallSubnet'
          }
          publicIPAddress: {
            id: afwPip.id
          }
        }
      }
    ]
    managementIpConfiguration: {
      name: 'mgmt-ipconfig'
      properties: {
        subnet: {
          id: '${hubVnet.id}/subnets/AzureFirewallManagementSubnet'
        }
        publicIPAddress: {
          id: afwMgmtPip.id
        }
      }
    }
  }
  dependsOn: [afwRuleCollectionGroup]
}

// =============================================================
//  Route Table — スポーク送信トラフィックを Firewall 経由に強制
// =============================================================
resource routeTable 'Microsoft.Network/routeTables@2023-11-01' = {
  name: 'rt-spoke-web-jpe-${suffix}'
  location: location
  properties: {
    routes: [
      {
        name: 'default-to-firewall'
        properties: {
          addressPrefix: '0.0.0.0/0'
          nextHopType: 'VirtualAppliance'
          nextHopIpAddress: '10.0.1.4'
        }
      }
    ]
  }
  dependsOn: [azFirewall]
}

// =============================================================
//  Spoke VNet (Route Table をサブネットに関連付け)
// =============================================================
resource spokeVnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: 'vnet-spoke-web-jpe-${suffix}'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: ['10.1.0.0/16']
    }
    subnets: [
      {
        name: 'snet-web-jpe'
        properties: {
          addressPrefix: '10.1.1.0/24'
          routeTable: { id: routeTable.id }
        }
      }
    ]
  }
}

// =============================================================
//  VNet ピアリング (Hub ↔ Spoke)
// =============================================================
resource hubToSpoke 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: hubVnet
  name: 'peer-hub-to-spoke'
  properties: {
    remoteVirtualNetwork: { id: spokeVnet.id }
    allowVirtualNetworkAccess: true
    allowForwardedTraffic: true
    allowGatewayTransit: false
    useRemoteGateways: false
  }
}

resource spokeToHub 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-11-01' = {
  parent: spokeVnet
  name: 'peer-spoke-to-hub'
  properties: {
    remoteVirtualNetwork: { id: hubVnet.id }
    allowVirtualNetworkAccess: true
    allowForwardedTraffic: true
    allowGatewayTransit: false
    useRemoteGateways: false
  }
}

// =============================================================
//  Standard 内部ロードバランサー
// =============================================================
resource lb 'Microsoft.Network/loadBalancers@2023-11-01' = {
  name: 'lbi-web-jpe-${suffix}'
  location: location
  sku: { name: 'Standard' }
  properties: {
    frontendIPConfigurations: [
      {
        name: 'frontend-web'
        properties: {
          subnet: { id: '${spokeVnet.id}/subnets/snet-web-jpe' }
          privateIPAllocationMethod: 'Dynamic'
        }
      }
    ]
    backendAddressPools: [
      { name: 'backend-web' }
    ]
    probes: [
      {
        name: 'probe-http'
        properties: {
          protocol: 'Http'
          port: 80
          requestPath: '/'
          intervalInSeconds: 15
          numberOfProbes: 2
        }
      }
    ]
    loadBalancingRules: [
      {
        name: 'rule-http'
        properties: {
          frontendIPConfiguration: {
            id: resourceId('Microsoft.Network/loadBalancers/frontendIPConfigurations', 'lbi-web-jpe-${suffix}', 'frontend-web')
          }
          backendAddressPool: {
            id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', 'lbi-web-jpe-${suffix}', 'backend-web')
          }
          probe: {
            id: resourceId('Microsoft.Network/loadBalancers/probes', 'lbi-web-jpe-${suffix}', 'probe-http')
          }
          protocol: 'Tcp'
          frontendPort: 80
          backendPort: 80
          idleTimeoutInMinutes: 4
        }
      }
    ]
  }
}

// =============================================================
//  Web VM — NIC (スポーク snet-web-jpe + LB バックエンド)
// =============================================================
resource nic01 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: 'nic-vm-web-01-jpe'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: { id: '${spokeVnet.id}/subnets/snet-web-jpe' }
          privateIPAllocationMethod: 'Dynamic'
          loadBalancerBackendAddressPools: [
            { id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', lb.name, 'backend-web') }
          ]
        }
      }
    ]
  }
}

resource nic02 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: 'nic-vm-web-02-jpe'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: { id: '${spokeVnet.id}/subnets/snet-web-jpe' }
          privateIPAllocationMethod: 'Dynamic'
          loadBalancerBackendAddressPools: [
            { id: resourceId('Microsoft.Network/loadBalancers/backendAddressPools', lb.name, 'backend-web') }
          ]
        }
      }
    ]
  }
}

// =============================================================
//  Web VM — Zone 1 (vm-web-01-jpe)
// =============================================================
resource vmWeb01 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: 'vm-web-01-jpe-${suffix}'
  location: location
  zones: ['1']
  properties: {
    hardwareProfile: { vmSize: 'Standard_B2s' }
    osProfile: {
      computerName: 'vm-web-01-jpe'
      adminUsername: adminUsername
      adminPassword: adminPassword
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: '0001-com-ubuntu-server-jammy'
        sku: '22_04-lts-gen2'
        version: 'latest'
      }
      osDisk: {
        createOption: 'FromImage'
        managedDisk: { storageAccountType: 'Premium_LRS' }
      }
    }
    networkProfile: {
      networkInterfaces: [{ id: nic01.id }]
    }
    diagnosticsProfile: {
      bootDiagnostics: { enabled: false }
    }
  }
}

resource vmWeb01CustomScript 'Microsoft.Compute/virtualMachines/extensions@2024-03-01' = {
  parent: vmWeb01
  name: 'install-nginx'
  location: location
  properties: {
    publisher: 'Microsoft.Azure.Extensions'
    type: 'CustomScript'
    typeHandlerVersion: '2.1'
    autoUpgradeMinorVersion: true
    settings: {
      script: base64('''#!/bin/bash
apt-get update -y
apt-get install -y nginx
ZONE=$(curl -s -H Metadata:true \
  'http://169.254.169.254/metadata/instance/compute/zone?api-version=2021-02-01&format=text')
echo "<h1>Hello from $(hostname) - Zone ${ZONE}</h1>" > /var/www/html/index.html
systemctl enable nginx
systemctl start nginx
''')
    }
  }
}

// =============================================================
//  Web VM — Zone 2 (vm-web-02-jpe)
// =============================================================
resource vmWeb02 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: 'vm-web-02-jpe-${suffix}'
  location: location
  zones: ['2']
  properties: {
    hardwareProfile: { vmSize: 'Standard_B2s' }
    osProfile: {
      computerName: 'vm-web-02-jpe'
      adminUsername: adminUsername
      adminPassword: adminPassword
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: '0001-com-ubuntu-server-jammy'
        sku: '22_04-lts-gen2'
        version: 'latest'
      }
      osDisk: {
        createOption: 'FromImage'
        managedDisk: { storageAccountType: 'Premium_LRS' }
      }
    }
    networkProfile: {
      networkInterfaces: [{ id: nic02.id }]
    }
    diagnosticsProfile: {
      bootDiagnostics: { enabled: false }
    }
  }
}

resource vmWeb02CustomScript 'Microsoft.Compute/virtualMachines/extensions@2024-03-01' = {
  parent: vmWeb02
  name: 'install-nginx'
  location: location
  properties: {
    publisher: 'Microsoft.Azure.Extensions'
    type: 'CustomScript'
    typeHandlerVersion: '2.1'
    autoUpgradeMinorVersion: true
    settings: {
      script: base64('''#!/bin/bash
apt-get update -y
apt-get install -y nginx
ZONE=$(curl -s -H Metadata:true \
  'http://169.254.169.254/metadata/instance/compute/zone?api-version=2021-02-01&format=text')
echo "<h1>Hello from $(hostname) - Zone ${ZONE}</h1>" > /var/www/html/index.html
systemctl enable nginx
systemctl start nginx
''')
    }
  }
}

// =============================================================
//  管理用 VM — NIC (ハブ snet-mgmt-jpe、パブリック IP なし)
// =============================================================
resource nicMgmt 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: 'nic-vm-mgmt-jpe-${suffix}'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: { id: '${hubVnet.id}/subnets/snet-mgmt-jpe' }
          privateIPAllocationMethod: 'Dynamic'
        }
      }
    ]
  }
}

// =============================================================
//  管理用 VM (vm-mgmt-jpe)
// =============================================================
resource vmMgmt 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: 'vm-mgmt-jpe-${suffix}'
  location: location
  properties: {
    hardwareProfile: { vmSize: 'Standard_B2s' }
    osProfile: {
      computerName: 'vm-mgmt-jpe-${suffix}'
      adminUsername: adminUsername
      adminPassword: adminPassword
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: '0001-com-ubuntu-server-jammy'
        sku: '22_04-lts-gen2'
        version: 'latest'
      }
      osDisk: {
        createOption: 'FromImage'
        managedDisk: { storageAccountType: 'Premium_LRS' }
      }
    }
    networkProfile: {
      networkInterfaces: [{ id: nicMgmt.id }]
    }
    diagnosticsProfile: {
      bootDiagnostics: { enabled: false }
    }
  }
}

// =============================================================
//  Azure Bastion Developer (専用サブネット不要)
// =============================================================
resource bastion 'Microsoft.Network/bastionHosts@2023-11-01' = {
  name: 'bas-hub-jpe-${suffix}'
  location: location
  sku: { name: 'Developer' }
  properties: {
    virtualNetwork: { id: hubVnet.id }
  }
}

// =============================================================
//  Log Analytics ワークスペース
// =============================================================
resource logWorkspace 'Microsoft.OperationalInsights/workspaces@2023-09-01' = {
  name: 'log-lab-handson-jpe-${suffix}'
  location: location
  properties: {
    sku: { name: 'PerGB2018' }
    retentionInDays: 30
  }
}

// =============================================================
//  Recovery Services コンテナー (ZRS)
// =============================================================
resource rsv 'Microsoft.RecoveryServices/vaults@2024-04-01' = {
  name: 'rsv-lab-handson-jpe-${suffix}'
  location: location
  sku: {
    name: 'RS0'
    tier: 'Standard'
  }
  properties: {
    redundancySettings: {
      standardTierStorageRedundancy: 'ZoneRedundant'
    }
  }
}

// Enhanced (V2) 日次バックアップポリシー — 7日保持
resource backupPolicy 'Microsoft.RecoveryServices/vaults/backupPolicies@2024-04-01' = {
  parent: rsv
  name: 'policy-vm-enhanced-daily-7d'
  properties: {
    backupManagementType: 'AzureIaasVM'
    policyType: 'V2'
    instantRpRetentionRangeInDays: 2
    schedulePolicy: {
      schedulePolicyType: 'SimpleSchedulePolicyV2'
      scheduleRunFrequency: 'Daily'
      dailySchedule: {
        scheduleRunTimes: ['2000-01-01T17:00:00Z'] // UTC 17:00 = JST 02:00
      }
    }
    retentionPolicy: {
      retentionPolicyType: 'LongTermRetentionPolicy'
      dailySchedule: {
        retentionTimes: ['2000-01-01T17:00:00Z']
        retentionDuration: {
          count: 7
          durationType: 'Days'
        }
      }
    }
    timeZone: 'Tokyo Standard Time'
  }
}

// =============================================================
//  Outputs
// =============================================================
output suffix string = suffix
output resourceGroupName string = resourceGroup().name
output hubVnetId string = hubVnet.id
output spokeVnetId string = spokeVnet.id
output firewallPrivateIp string = '10.0.1.4'
output firewallPublicIp string = afwPip.properties.ipAddress
output lbName string = lb.name
output logWorkspaceId string = logWorkspace.properties.customerId
output rsvId string = rsv.id
