import api from "./axios";

// NB! JWT auth is done through Bearer header, so fetching the avatar thorugh
// <img src=...> won't work. 
// Solution here is to get it as a blob

export async function fetchAvatarBlob(userId: number): Promise<Blob> {
  // ts busts the cache (eg. after upload)
  // If not done, the browser cache causes the image visible to the user to not change
  // The other option would be to use version numbering.
  const ts = Date.now();
  const res = await api.get(`/avatars/${userId}?ts=${ts}`, { responseType: "blob" });
  return res.data;
}
