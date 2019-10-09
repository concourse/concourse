module PinnedTests exposing (all)

import Expect
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import Test exposing (Test, describe, test)


all : Test
all =
    describe "Pinned"
        [ test
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
        , test "startPinningTo allows switching without unpinning" <|
            \_ ->
                PinnedDynamicallyTo
                    { comment = ""
                    , pristineComment = ""
                    }
                    0
                    |> Pinned.startPinningTo 1
                    |> Expect.equal (Switching { comment = "", pristineComment = "" } 0 1)
        ]
