module Resource.Text exposing
  ( resourceLabel )

import Resource.Models as Models exposing (Model)

resourceLabel : Model -> String
resourceLabel model =
   model.name ++ " (" ++ model.type_ ++ ")"
