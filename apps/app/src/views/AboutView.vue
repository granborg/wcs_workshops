<script setup lang="ts">
// How the app works, for new users. 
</script>

<template>
  <article class="about">
    <section class="intro card">
      <h2>About WCS Workshops</h2>
      <p>WCS Workshops is a place to watch West Coast Swing workshop videos. Creators upload videos and share them with you through QR codes. Scan a code, and the video or folder is added to your library.</p>
    </section>

    <section class="card">
      <h2>Two kinds of accounts</h2>
      <ul>
        <li><strong>Viewers</strong> watch videos that creators share with them.</li>
        <li><strong>Creators</strong> upload videos and share them with viewers and other creators. Creators can also watch videos that other creators share with them.</li>
      </ul>
      <p>You choose your account type in Settings.</p>
    </section>

    <section class="card">
      <h2><i class="pi pi-book" aria-hidden="true" /> Library</h2>
      <p>The library is the main page. It is a list of all of the creators who have shared with you. Clicking on each opens that creator's collection of videos.</p>
      <p>The videos under each creator are organized in whatever way the creator has organized their own videos.</p>
      <p>A video with the tag "Not uploaded yet" is a placeholder: the creator has shared it but has not uploaded the file yet. It plays when the creator uploads it. You do not have to scan the code again.</p>
      <p>If you are a creator, your own collection shows in bold, and you have administrative options for all of the videos and folders in it.</p>
    </section>

    <section class="card">
      <h2><i class="pi pi-qrcode" aria-hidden="true" /> Scan</h2>
      <p>This is where you scan a QR code to add a video or a folder to your library. You can also paste a share link that a creator sent you. Scanning a code with your phone's camera app works too.</p>
      <p>Once you have access, it lasts until the creator revokes your access or removes the video or folder you have access to.</p>
      <ul>
        <li>If you have access to an individual video, you can see that video no matter where the creator puts it in their collection.</li>
        <li>If you have access to a folder, you can see any video in that folder or its sub-folders.</li>
      </ul>
    </section>

    <section class="card">
      <h2><i class="pi pi-cog" aria-hidden="true" /> Settings</h2>
      <ul>
        <li><strong>Name:</strong> You can change your name at any time. Viewers see a creator's name in their library. Creators see viewer names when they audit who has access.</li>
        <li><strong>Account type:</strong> Creator or Viewer. To change from creator to viewer, you must first delete your entire video collection.</li>
        <li><strong>Appearance:</strong> Light, dark, or the same as your device.</li>
      </ul>
    </section>

    <h2 class="part card">For creators</h2>

    <section class="card">
      <h2><i class="pi pi-upload" aria-hidden="true" /> Upload</h2>
      <p>This is where you add a video to a folder in your collection. At the top, choose <strong>Upload now</strong> or <strong>Placeholder</strong>.</p>

      <h3>Upload now</h3>
      <p>Choose the video file, a title, and the destination folder. The file goes directly from your device to Vimeo, which hosts the videos. Keep the app open until the upload finishes.</p>
      <p>If an upload did not finish or failed, open the video and upload the file again on its page.</p>

      <h3>Placeholder</h3>
      <p>Use a placeholder when the video is not ready yet. A placeholder needs only a title and a folder. It shows in your collection with the tag "Not uploaded yet", and you can share it at once.</p>
      <p>When the file is ready, open the placeholder and upload the file on its page. Everyone who already has access sees the video then, and you do not have to share it again. This is useful for giving people access to a workshop video at the time of the lesson, and then uploading it for everyone later.</p>
    </section>

    <section class="card">
      <h2><i class="pi pi-folder" aria-hidden="true" /> Organizing your collection</h2>
      <p>You build and organize your video collection in folders. You can create, rename, and remove folders. Renaming a folder does not change who has access to it.</p>
      <p>When you remove a folder with videos in it, you are asked whether to delete all the videos or move them to another folder.</p>
    </section>

    <section class="card">
      <h2><i class="pi pi-ellipsis-v" aria-hidden="true" /> Options for each folder and video</h2>
      <p>Each folder and each video in your collection has an options button with these choices:</p>
      <dl>
        <dt>Share (QR code)</dt>
        <dd>Opens a QR code for the folder or video. If one was already created, you can show, print, or copy the link of the existing one. You can also generate a new one and optionally set an expiration date and time. The QR code grants any user who scans it access to that folder or video. It is added to their library.</dd>
        <dt>View / edit access</dt>
        <dd>See which users have been granted access to the item, and revoke access for any user.</dd>
        <dt>Rename</dt>
        <dd>Change the name. Access stays the same.</dd>
        <dt>Move</dt>
        <dd>Move a folder or video to another folder. This can change who has access: access given on the item itself follows it, and access that came from the old folder stops.</dd>
        <dt>Delete</dt>
        <dd>Delete a video or folder. Deleted videos are also removed from Vimeo. This cannot be undone.</dd>
      </dl>
    </section>

    <section class="card">
      <h2><i class="pi pi-history" aria-hidden="true" /> Audit log</h2>
      <p>Open it from Settings. It is a timestamped record of who (and from which IP address) was granted access to which video or folder, which QR code was used, when access was removed, and when a QR code was generated or revoked.</p>
    </section>
  </article>
</template>

<style scoped>
.about {
  padding-bottom: 24px;
}

.intro {
  font-size: 1.0625rem;
  margin-bottom: 20px;
}

section {
  margin-bottom: 24px;
}

section h2 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 1.0625rem;
  margin-bottom: 10px;
}

section h3 {
  margin: 16px 0 6px;
  font-size: 0.9375rem;
  font-weight: 600;
}

section h2 .pi {
  color: var(--p-primary-color);
}

.part {
  margin: 28px 0 12px;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--p-text-muted-color);
}

p:last-child,
ul:last-child,
dl:last-child {
  margin-bottom: 0;
}

ul {
  margin: 0 0 12px;
  padding-left: 20px;
}

li + li {
  margin-top: 6px;
}

dl {
  margin: 0;
}

dt {
  font-weight: 600;
  margin-top: 10px;
}

dt:first-child {
  margin-top: 0;
}

dd {
  margin: 2px 0 0;
  color: var(--p-text-muted-color);
}
</style>
