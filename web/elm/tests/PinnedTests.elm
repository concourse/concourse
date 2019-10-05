module PinnedTests exposing (all)

import Expect
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import Test exposing (Test, test)


all : Test
all =
    test
        ("when resource is dynamically pinned, other versions are "
            ++ "NotThePinnedVersion"
        )
    <|
        \_ ->
            Pinned.pinState 1
                0
                (PinnedDynamicallyTo
                    { comment = ""
                    , pristineComment = ""
                    }
                    0
                )
                |> Expect.equal NotThePinnedVersion
