// location-map.js couples the map picker on the location form with the
// latitude and longitude fields in both directions. It is only an input aid:
// the core checks the coordinates again when the form is saved, and without
// this script (or without Leaflet) the map stays hidden and the fields work
// on their own.
(function () {
  "use strict";

  const MAP_ID = "location-map";
  const LATITUDE_ID = "latitude";
  const LONGITUDE_ID = "longitude";

  // Without valid field values the map shows the centre of Zirndorf.
  const ZIRNDORF_CENTRE = [49.4424, 10.9539];
  const OVERVIEW_ZOOM = 14;
  const MARKER_ZOOM = 17;

  const TILE_URL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png";
  const TILE_MAX_ZOOM = 19;
  const TILE_ATTRIBUTION =
    '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap-Mitwirkende</a>';

  const LATITUDE_LIMIT = 90;
  const LONGITUDE_LIMIT = 180;
  const COORDINATE_DECIMALS = 6;

  const DECIMAL_COMMA = /,/g;
  // Plain decimal notation only: a deliberately stricter subset of what the
  // core accepts (Go's strconv.ParseFloat also reads hex floats such as
  // "0x1p4"). It keeps out what Number() would accept, such as "0x10" or
  // "Infinity".
  const DECIMAL_NUMBER = /^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i;

  const INVALID_ATTRIBUTE = "aria-invalid";

  // parseCoordinate reads a field value like the admin does: decimal comma
  // allowed, finite and within ±limit inclusive. It returns null for an empty
  // field and NaN for an invalid one.
  function parseCoordinate(text, limit) {
    const value = text.trim().replace(DECIMAL_COMMA, ".");
    if (value === "") {
      return null;
    }
    if (!DECIMAL_NUMBER.test(value)) {
      return NaN;
    }
    const number = Number(value);
    if (!Number.isFinite(number) || number < -limit || number > limit) {
      return NaN;
    }
    return number;
  }

  // fieldCoordinates returns the position in both fields, or null unless
  // both hold a valid coordinate.
  function fieldCoordinates(latitudeField, longitudeField) {
    const latitude = parseCoordinate(latitudeField.value, LATITUDE_LIMIT);
    const longitude = parseCoordinate(longitudeField.value, LONGITUDE_LIMIT);
    if (latitude === null || longitude === null || Number.isNaN(latitude) || Number.isNaN(longitude)) {
      return null;
    }
    return L.latLng(latitude, longitude);
  }

  // markFieldValidity flags a field whose value is present but invalid and
  // clears the flag otherwise.
  function markFieldValidity(field, limit) {
    if (Number.isNaN(parseCoordinate(field.value, limit))) {
      field.setAttribute(INVALID_ATTRIBUTE, "true");
    } else {
      field.removeAttribute(INVALID_ATTRIBUTE);
    }
  }

  function initLocationMap() {
    if (typeof L === "undefined") {
      return;
    }
    const mapArea = document.getElementById(MAP_ID);
    const latitudeField = document.getElementById(LATITUDE_ID);
    const longitudeField = document.getElementById(LONGITUDE_ID);
    if (!mapArea || !latitudeField || !longitudeField) {
      return;
    }

    // The map area is hidden in the HTML so that it never shows up empty
    // without this script; Leaflet needs it visible to measure its size.
    mapArea.hidden = false;
    const map = L.map(mapArea);
    L.tileLayer(TILE_URL, { maxZoom: TILE_MAX_ZOOM, attribution: TILE_ATTRIBUTION }).addTo(map);

    let marker = null;

    function writeFields(position) {
      latitudeField.value = position.lat.toFixed(COORDINATE_DECIMALS);
      longitudeField.value = position.lng.toFixed(COORDINATE_DECIMALS);
      latitudeField.removeAttribute(INVALID_ATTRIBUTE);
      longitudeField.removeAttribute(INVALID_ATTRIBUTE);
    }

    function placeMarker(position) {
      if (marker) {
        marker.setLatLng(position);
        return;
      }
      marker = L.marker(position, { draggable: true }).addTo(map);
      marker.on("dragend", function () {
        // Wrap once so marker and fields show the same longitude.
        const position = marker.getLatLng().wrap();
        marker.setLatLng(position);
        writeFields(position);
      });
    }

    const initial = fieldCoordinates(latitudeField, longitudeField);
    if (initial) {
      map.setView(initial, MARKER_ZOOM);
      placeMarker(initial);
    } else {
      map.setView(ZIRNDORF_CENTRE, OVERVIEW_ZOOM);
    }

    map.on("click", function (event) {
      // Wrap once so marker and fields show the same longitude.
      const position = event.latlng.wrap();
      placeMarker(position);
      writeFields(position);
    });

    // "change" instead of "input": half-typed values such as "49," must not
    // move the marker.
    function followField(field, limit) {
      field.addEventListener("change", function () {
        markFieldValidity(field, limit);
        const position = fieldCoordinates(latitudeField, longitudeField);
        if (position) {
          placeMarker(position);
          map.panTo(position);
        }
      });
    }
    followField(latitudeField, LATITUDE_LIMIT);
    followField(longitudeField, LONGITUDE_LIMIT);
  }

  initLocationMap();
})();
