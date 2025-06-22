local statePath = std.extVar('STATE_PATH');
local replica = std.extVar('REPLICA');
local shard = std.extVar('SHARD');

{
  grpcServers: [{
    listenPaths: ['%s/bonanza_storage_shard_%s%s.sock' % [statePath, replica, shard]],
    authenticationPolicy: { allow: {} },
  }],

  leasesMapRecordsCount: 1e6,
  leasesMapLeaseCompletenessDuration: '120s',
  leasesMapMaximumGetAttempts: 16,
  leasesMapMaximumPutAttempts: 64,

  local dataPath = '%s/bonanza_storage_shard_%s%s' % [statePath, replica, shard],
  localObjectStore: {
    referenceLocationMapOnBlockDevice: { file: {
      path: dataPath + '/reference_location_map',
      sizeBytes: 1e8,
    } },
    referenceLocationMapMaximumGetAttempts: 16,
    referenceLocationMapMaximumPutAttempts: 64,

    locationBlobMapOnBlockDevice: { file: {
      path: dataPath + '/location_blob_map',
      sizeBytes: 1e10,
    } },

    oldRegionSizeRatio: 1,
    currentRegionSizeRatio: 1,
    newRegionSizeRatio: 3,

    persistent: {
      stateDirectoryPath: dataPath + '/persistent_state',
      minimumEpochInterval: '300s',
    },
  },
}
