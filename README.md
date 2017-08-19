With respect to Redis, the following are samples of how the compiled data is to be stored:
```
compiled:{pub\_id}:meta:static:[...properties]
compiled:{pub\_id}:meta:dynamic:[...properties]
compiled:{pub\_id}:e:{entity\_enum}:{entity\_id}
```

The following static properties are used as metadata in the smartspeaker
compiled:{pub\_id}:meta:static:name string
compiled:{pub\_id}:meta:static:authors set
compiled:{pub\_id}:meta:dynamic:plays integer

The following are valid entities
```
Zone int8 = 1 << iota
Actor
Dialog
Trigger
```

Zones
  compiled:{pid}:0
  -- Dialogs --
  -- The following is a List of out-logic node IDs
  -- Human readable version compiled:{pub\_id}:{Zones}:{zone\_id}:entities:{Dialogs}:input
  compiled:{pid}:e:0:{zid}:e:2:i:[...inputs\_compiled] binary\_outlogic

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
run(dialog\_action\_bundle\_id\_4)
if ((global.foo === "bar" && global.hello === "world") || global.count > 100) {
	run(dialog\_action\_bundle\_id\_1000)
} else if (global.bar === "foo" && global.world === "hello" && global.count <= 100) {
	run(dialog\_action\_bundle\_id\_2000)
} else {
    run(dialog\_action\_bundle\_id\_3000)
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



TODO:
- Better define triggers
- Compile triggers
- Handle dialog node cycles (Create a "IsRoot" bool?)
- Handle comparison logic
- Implement additional ARAs