local statePath = std.extVar('STATE_PATH');

{
  httpServers: [{
    listenAddresses: [':9982'],
    authenticationPolicy: { allow: {} },
  }],
  storageGrpcClient: {
    address: 'unix://%s/bonanza_storage_frontend.sock' % statePath,
  },
}
