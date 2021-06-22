// addEventListeners is a helper that adds the provided function as a listener
// to all provided targets and events.
//
// Simple example:
//
//   addEventListeners(document, 'load', () => { ... });
//
// With multiple elements:
//
//   let targets = [button1, button2];
//   addEventListeners(elements, 'click', () => { ... });
//
// With multiple events:
//
//   let elements = [username, password];
//   addEventListeners(elements, 'change keyup paste', () => { ... });
//   addEventListeners(elements, ['change', 'keyup', 'paste'], () => { ... });
//
const addEventListeners = (givenElements, givenEvents, fn) => {
  const elements = flattenArray(givenElements);
  const events = flattenArray(
    typeof givenEvents === 'string' ? givenEvents.split(' ').map((s) => s.trim()) : givenEvents
  );

  elements.forEach((element) => {
    events.forEach((event) => {
      element.addEventListener(event, fn.bind(element), false);
    });
  });
};

// flattenArray flattens the given array. If the element is not an array, it is
// put as a single element in the array.
const flattenArray = (arr) => {
  return [].concat.apply([], [arr]);
};

// clearChildren removes all children from the given element.
const clearChildren = (element) => {
  while (element && element.firstChild) {
    element.removeChild(element.firstChild);
  }
};

// matchesSelector returns true if the given element matches the provided
// selector, or false otherwise.
const matchesSelector = (element, selector) => {
  return (
    element.matches ||
    element.matchesSelector ||
    element.msMatchesSelector ||
    element.mozMatchesSelector ||
    element.webkitMatchesSelector ||
    element.oMatchesSelector
  ).call(element, selector);
};

// uploadWithRetries attempts to upload using the provided uploadFn retrying 3
// times.
const uploadWithRetries = async (uploadFn) => {
  // Common error codes which should cancel the whole upload.
  const stopUploadingCodes = [
    '403', // forbidden
    '404', // not-found
    '503', // unavailable
  ];

  // stopUploadingEnum are the error values which should immediately terminate
  // uploading.
  const stopUploadingEnum = ['maintenance_mode', 'sms_queue_full'];

  let cancel = false;
  for (let retries = 3; retries > 0; retries--) {
    await uploadFn()
      .then(() => {
        retries = 0;
      })
      .catch(async function (err) {
        if (!err) {
          return;
        }
        if (err.responseJSON && stopUploadingEnum.includes(err.responseJSON.errorCode)) {
          flash.alert('Status ' + err.responseJSON.errorCode + ' detected. Canceling remaining upload.');
          cancel = true;
          retries = 0;
        } else if (stopUploadingCodes.includes(err.status)) {
          flash.alert('Code ' + err.status + ' detected. Canceling remaining upload.');
          cancel = true;
          retries = 0;
        } else {
          // Throttling
          let after = err.getResponseHeader('retry-after');
          if (after) {
            let sleep = new Date(after) - new Date();
            if (sleep > 0) {
              flash.alert('Rate limited. Sleeping for ' + (sleep + 100) / 1000 + 's.');
              await new Promise((r) => setTimeout(r, sleep + 100));
            }
          } else {
            retries = 0;
          }
        }
      });
  }
  return cancel;
};
