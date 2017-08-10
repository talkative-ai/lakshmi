With respect to Redis, the following are samples of how the compiled data is to be stored:
```
compiled:{game_id}:meta:static:[...properties]
compiled:{game_id}:meta:static:[...dynamic]
compiled:{game_id}:e:{entity_enum}:{entity_id}
```

The following are valid entities
```
Zone int8 = 1 << iota
Actor
Dialog
Trigger
```

Zones
  compiled:{gid}:0
  -- Dialogs --
  -- The following is a List of out-logic node IDs
  -- Human readable version compiled:{game_id}:{Zones }:{zone_id}:entities:{Dialogs}:input
  compiled:{gid}:e:0:{zid}:e:2:i:[...inputs_compiled]
  -- The following contains an out-logic node structure as a string
  -- Human readable format compiled:{game_id}:entities:{entity_enum=0}:{zone_id}:entities:{entity_enum=2}:out:{outlogic_id}
  compiled:{gid}:e:0:{zid}:e:2:o:{outlogic_id}

```
Out-logic node
Conditions are an array of objects
Each object in the array is an "Or" node
Each parallel prop within the "conditions" objects are an "And" node
```

Therefore

```json
{
   "always": 4000,
   "statements": [
       [{
           "conditions": [{
               "eq": {
                   "123": "bar",
                   "456": "world"
               },
               "gt": {
                   "789": 100
               }
           }],
           "then": [
               1000
           ]
       }, {
           "conditions": [{
               "eq": {
                   "321": "foo",
                   "654": "hello"
               },
               "lte": {
                   "1231": 100
               }
           }],
           "then": [
               2000
           ]
       }, {
           "then": [
               3000
           ]
       }]
   ]
}
```

This object can be read as the following pseudocode
(In practice it will never truly look like this, however):
```js
run(dialog_action_bundle_id_4)
if ((global.foo === "bar" && global.hello === "world") || global.count > 100) {
	run(dialog_action_bundle_id_1000)
} else if (global.bar === "foo" && global.world === "hello" && global.count <= 100) {
	run(dialog_action_bundle_id_2000)
} else {
    run(dialog_action_bundle_id_3000)
}
```

Note that the integer key values are actually variable IDs generated in an earlier phase of the compilation process.
The same is with the "then" and "always" values, where the integers are IDs that represent action chunks.

Ultimately these logical values are compiled down into a simple byte stream.
The backend will then convert it to a byte stream:

- uint8 boolean "has AlwaysExec"
- uint64 "AlwaysExec" id
- uint8 number of statements arrasys
- (for each statements array)
  - uint32 number of bytes within statements array
  - uint8 number of statements within statements array
  - uint8 number of statement conditions (number of "or" conditions)
  - (for each "or" group)
    - uint8 expected operator, OR'd together
        - operators: eq = 1 << iota, lt, gt, le, ge, ne
    - (for each sibling operator [treat as and])
        - uint16 number of inner comparisons
        - (for each comparison)
            - uint64 variable name
            - uint8 value type
            - uint16 buffer length (if necessary)
            - value
