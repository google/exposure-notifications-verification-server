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
