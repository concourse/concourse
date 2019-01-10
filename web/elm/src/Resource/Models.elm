module Resource.Models exposing (Hoverable(..), VersionToggleAction(..))


type Hoverable
    = PreviousPage
    | NextPage
    | CheckButton
    | None


type VersionToggleAction
    = Enable
    | Disable
