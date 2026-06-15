// Configure React testing environment
(globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true

// Mock ResizeObserver for jsdom
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as Record<string, unknown>).ResizeObserver = MockResizeObserver

// Mock scrollIntoView for jsdom
if (typeof Element !== 'undefined') {
  Element.prototype.scrollIntoView = function () {}
}

if (typeof window !== 'undefined' && !window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() { return false },
    }),
  })
}
