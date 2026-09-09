// The script of the application.
//
// Datastar reads the data- attributes of the page and patches an element from
// the answer of the server. `avero js pin` fetched it as a bundled ES module
// and recorded its address and its hash in avero.lock, so the build needs no
// network and no Node.js. See DX-9 and S12.
import "datastar";
import "../vendor/basecoat/js/all.min.js";

// Write the behaviour that Datastar does not cover here. Run
// `avero js pin <package> <url>` to add a module, and import it by its name.
