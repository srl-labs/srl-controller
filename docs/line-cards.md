# Line cards

Modular SR Linux chassis (e.g. `ixr6e`, `ixr10e`) can be emulated with more than one line card by setting
`spec.components` on the `Srlinux` resource. Chassis variants require a [license](using-licenses.md).

```yaml
apiVersion: kne.srlinux.dev/v1
kind: Srlinux
metadata:
  name: r1
spec:
  model: ixr10e
  components:
    - slot: 1 # type omitted: line card of the model's topology template
    - slot: 2
      type: imm60-100g-qsfp28
```

When `components` is set, the controller renders a per-node config map `<name>-topology` from the model's
entry in the `srlinux-variants` config map, replacing its `slot_configuration` with one entry per component:

- A component without `type` uses the line card of the lowest numbered slot in the model's template.
- A component with `type` uses that line card. If any component has a `type`, the chassis CPM is set from the
  first such component.

When `components` is empty, the model's template is used unchanged.

The CRD rejects duplicate slots, slots below 1, unknown line card types, `components` on models other than `ixr6`,
`ixr10`, `ixr6e` and `ixr10e`, and slots outside the chassis (1-4 on `ixr6` and `ixr6e`, 1-8 on `ixr10` and
`ixr10e`).

To support components on another model, add it and its number of line card slots to the map in the
`+kubebuilder:validation:XValidation` marker on `SrlinuxSpec` in `api/v1/srlinux_types.go`, update the marker's
message, and run `make manifests`.

## Supported line card types

`type` is restricted by the CRD to the line cards in the catalog in
[`controllers/imm.go`](../controllers/imm.go). The card, MDA and CPM type IDs are taken from the
[containerlab](https://github.com/srl-labs/containerlab) SR Linux topology templates (`nodes/srl/topology/*.yml`).
The catalog holds 7250 IXR-e line cards and has been tested on `ixr6e` and `ixr10e`. On `ixr6` and `ixr10`, use
components without `type`.

To add a line card:

1. Add its IDs to `immCatalog` in `controllers/imm.go`.
2. Add its name to the `+kubebuilder:validation:Enum` marker on `IMMType` in `api/v1/srlinux_types.go`.
3. Run `make manifests`.

`go test ./controllers/...` fails if the CRD enum and the catalog differ.

The CRD and the controller image must be deployed together, as both carry the catalog.
