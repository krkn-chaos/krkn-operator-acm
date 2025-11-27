# Overview 
this is an operator used to integrate with RedHat ACM (Advanced Cluster Management) and a custom made operator
called krkn-operator.

# Requirements

- the operator must react to the creation of a CRD called `KrknTargetRequest`, this CRD contains a UUID, a targetData field
  (that will be a json that I'll describe later) and a status field
- the `KrknTargetRequest` CR is created by another operator with a valorized UUID and status to `pending`

once the operator receives the `KrknTargetRequest` the operator must:

- make an api request to  /apis/cluster.open-cluster-management.io/v1/managedclusters, the response payload can be found
  in misc/managed-cluster.json. This Api request goal is to list and collect the available 
  - cluster names (jsonpath .items[].name)
  - cluster API URL (jsonpath .items[].spec.managedClusterClientConfigs[].url)
  - cluster CA Bundle (jsonpath .items[].spec.managedClusterClientConfigs[].caBunble)
  - Each cluster name will have a namespace with the same name of the cluster containing a secret called `application-manager`
    - `application-manager` secret has two fields base64 encoded
      - ca.crt (containing the ca certificate of the cluster api)
      - token (the token that is used to query the managed cluster API)
- with all the information gathered must build a valid kubeconfig per each cluster
- create a secret which name is the UUID of the request
  - the secret must have a field .data.managed-clusters
    - the data must be a json map key with the name of the cluster and value an object 
    - the object must contain the cluster-name, cluster-api and kubeconfig (base64 encoded)
- when the secret is created the `KrknTargetRequest` status must be changed to Completed and the field targetData will contain a list of:
  - cluster-name
  - cluster-api-url



  
