local statePath = std.extVar('STATE_PATH');

{
  httpServers: [{
    listenAddresses: [':9982'],
    authenticationPolicy: { allow: {} },
  }],
  buildQueueStateGrpcClient: {
    address: 'unix://%s/bonanza_scheduler_buildqueuestate.sock' % statePath,
  },
  storageGrpcClient: {
    address: 'unix://%s/bonanza_storage_frontend.sock' % statePath,
  },
  parsedObjectPool: {
    cacheReplacementPolicy: 'LEAST_RECENTLY_USED',
    count: 1e5,
    sizeBytes: 1e8,
  },
}
