'use client';

import * as echarts from 'echarts/core';

import worldGeoJson from './data/world-geo.json';

type WorldFeature = {
  properties?: {
    name?: string;
  };
  geometry?: {
    type?: 'Polygon' | 'MultiPolygon';
    coordinates?: number[][][] | number[][][][];
  };
};

type WorldGeoJson = {
  type: string;
  features?: WorldFeature[];
};

let worldMapRegistrationAttempted = false;
let worldMapRegistrationSucceeded = false;

export function ensureWorldMapRegistered(): boolean {
  if (worldMapRegistrationSucceeded || echarts.getMap('world')) {
    worldMapRegistrationSucceeded = true;
    return true;
  }

  if (worldMapRegistrationAttempted) {
    return false;
  }

  worldMapRegistrationAttempted = true;

  try {
    const geoJson = worldGeoJson as WorldGeoJson;

    if (
      geoJson.type !== 'FeatureCollection' ||
      !Array.isArray(geoJson.features)
    ) {
      throw new Error('invalid world geojson payload');
    }

    echarts.registerMap(
      'world',
      geoJson as unknown as Parameters<typeof echarts.registerMap>[1],
    );

    if (!echarts.getMap('world')) {
      throw new Error('world map registration failed');
    }

    worldMapRegistrationSucceeded = true;
    return true;
  } catch (error) {
    const registrationError =
      error instanceof Error
        ? error
        : new Error('unknown world map registration error');
    console.error('Failed to register ECharts world map', registrationError);
    return false;
  }
}
