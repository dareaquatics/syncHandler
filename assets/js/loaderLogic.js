document.addEventListener("DOMContentLoaded", function () {
  const loadingScreen = document.getElementById("loading-screen");
  const content = document.getElementById("content");

  function showContent() {
    // Add class to fade out the loading screen
    loadingScreen.classList.add("fade-out");

    // Wait for the fade-out transition to finish before hiding
    setTimeout(() => {
      loadingScreen.style.display = "none";
      // Show and fade in the content
      content.style.display = "block";
      requestAnimationFrame(() => {
        content.style.opacity = "1";
      });
      document.body.style.overflow = "auto"; // Enable scrolling after loading
    }, 1000); // Matches the CSS transition duration for loading screen
  }

  // Generate a random loading time between 200 ms and 900 ms
  const minLoadingTime = 100; // Minimum loading time
  const maxLoadingTime = 300; // Maximum loading time
  const randomLoadingTime =
    Math.floor(Math.random() * (maxLoadingTime - minLoadingTime + 1)) +
    minLoadingTime;

  // Start the loading process with the random duration
  setTimeout(() => {
    // Ensure the loading animation continues for an additional second after loading
    setTimeout(() => {
      showContent();
    }, 300); // Keep animation for 1 second after loading
  }, randomLoadingTime); // Random loading time between 200 ms and 900 ms
});
