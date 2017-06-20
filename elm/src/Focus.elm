module Focus exposing (Focus, get, set, update, (=>), create)

{-| Our goal is to update a field deep inside some nested records. For example,
if we want to add one to `object.physics.velocity.x` or set it to zero, we would
be writing code like this:
update (physics => velocity => x) (\x -> x + 1) object
set (physics => velocity => x) 0 object
This means you could avoid writing record update syntax which would be messier.
**Warning!** It is possible that the concept of a `Focus` is harmful to code
quality in that it can help you to be lax with abstraction boundaries.
By making it easy to look deep inside of data structures, it encourages you to
stop thinking about how to make these substructures modular, perhaps leading
to messier architecture *and* some extra conceptual complexity. It may also
make your code slower by encouraging you to take many passes over data,
creating lots of intermediate data structures for no particular reason.
*Use with these risk in mind!*


# Focus

@docs Focus


# Get, Set, Update

@docs get, set, update


# Compose Foci

@docs (=>)


# Create your own Focus

@docs create

-}


{-| A `Focus` lets you focus on a small part of some larger data structure.
Maybe this means a certain field in a record or a certain element in an array.
The focus then lets you `get`, `set`, and `update` this small part of a big
value.
-}
type Focus big small
    = Focus
        { get : big -> small
        , update : (small -> small) -> big -> big
        }


{-| A `Focus` is a value. It describes a strategy for getting and updating
things. This function lets you define a `Focus` yourself by providing a `get`
function and an `update` function.
-}
create : (big -> small) -> ((small -> small) -> big -> big) -> Focus big small
create get update =
    Focus { get = get, update = update }


{-| Get a small part of a big thing.
x : Focus { record | x:a } a
get x { x=3, y=4 } == 3
Seems sort of silly given that you can just say `.x` to do the same thing. It
will become much more useful when we can begin to compose foci, so keep reading!
-}
get : Focus big small -> big -> small
get (Focus focus) big =
    focus.get big


{-| Set a small part of a big thing.
x : Focus { record | x:a } a
set x 42 { x=3, y=4 } == { x=42, y=4 }
-}
set : Focus big small -> small -> big -> big
set (Focus focus) small big =
    focus.update (always small) big


{-| Update a small part of a big thing.
x : Focus { record | x:a } a
update x sqrt { x=9, y=10 } == { x=3, y=10 }
This lets us chain updates without any special record syntax:
x : Focus { record | x:a } a
y : Focus { record | y:a } a
point
|> update x sqrt
|> update y sqrt
The downside of this approach is that this means we take two passes over the
record, whereas normal record syntax would only have required one. It may be
best to use a mix `Focus` and typical record updates to minimize traversals.
-}
update : Focus big small -> (small -> small) -> big -> big
update (Focus focus) f big =
    focus.update f big



-- COMPOSING FOCI


{-| The power of this library comes from the fact that you can compose many
foci. This means we can update a field deep inside some nested records. For
example, perhaps we want to add one to `object.physics.velocity.x` or set it to
zero.
physics : Focus { record | physics : a } a
velocity : Focus { record | velocity : a } a
x : Focus { record | x : a } a
y : Focus { record | y : a } a
update (physics => velocity => x) (\x -> x + 1) object
set (physics => velocity => x) 0 object
This would be a lot messier with typical record update syntax! This is what
makes this library worthwhile, but also what makes it dangerous. You will be
doing a lot of silly work if you start writing code like this:
object
|> set (physics => velocity => x) 0
|> set (physics => velocity => y) 0
It is pretty, but you pay for it in performance because you take two passes
over `object` instead of one. It may be best to do the last step with typical
record updates so that this can be done in one pass.
-}
(=>) : Focus big medium -> Focus medium small -> Focus big small
(=>) (Focus largerFocus) (Focus smallerFocus) =
    let
        get big =
            smallerFocus.get (largerFocus.get big)

        update f big =
            largerFocus.update (smallerFocus.update f) big
    in
        Focus { get = get, update = update }
