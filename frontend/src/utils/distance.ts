// Utility functions for calculating distance between geographic coordinates

/**
 * Calculate the distance between two points using the Haversine formula
 * @param lat1 Latitude of first point
 * @param lon1 Longitude of first point  
 * @param lat2 Latitude of second point
 * @param lon2 Longitude of second point
 * @returns Distance in meters
 */
export function calculateDistance(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371000; // Earth radius in meters
  const dLat = (lat2 - lat1) * (Math.PI / 180);
  const dLon = (lon2 - lon1) * (Math.PI / 180);
  const rLat1 = lat1 * (Math.PI / 180);
  const rLat2 = lat2 * (Math.PI / 180);

  const a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.sin(dLon / 2) * Math.sin(dLon / 2) * Math.cos(rLat1) * Math.cos(rLat2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

  return R * c; // Distance in meters
}

/**
 * Calculate distance if both locations are available
 * @param userLat Current user's latitude
 * @param userLon Current user's longitude
 * @param targetLat Target user's latitude  
 * @param targetLon Target user's longitude
 * @returns Distance in meters, or undefined if any coordinate is missing
 */
export function calculateDistanceIfAvailable(
  userLat?: number,
  userLon?: number,
  targetLat?: number,
  targetLon?: number
): number | undefined {
  if (userLat == null || userLon == null || targetLat == null || targetLon == null) {
    return undefined;
  }
  return calculateDistance(userLat, userLon, targetLat, targetLon);
}